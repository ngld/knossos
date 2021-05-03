package mods

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/davecgh/go-spew/spew"
	"github.com/rotisserie/eris"

	"github.com/ngld/knossos/packages/server/pkg/db/queries"
	"github.com/ngld/knossos/packages/server/pkg/nblog"
)

type (
	DependencySnapshot map[string]string
	constraintItem     struct {
		constraint *semver.Constraints
		preReq     map[string]string
		source     string
	}
)

type modRequest struct {
	modid   string
	version string
	preReq  map[string]string
	source  string
}

type queryDependencyList []struct {
	Modid    string
	Version  string
	Packages []string
}

func (item constraintItem) String() string {
	return fmt.Sprintf("%s (%s)", item.constraint, item.source)
}

func pickNaiveVersion(ctx context.Context, available []string, constraints *semver.Constraints) (string, error) {
	parsedVersions := make(semver.Collection, len(available))
	for idx, rawVer := range available {
		ver, err := semver.StrictNewVersion(rawVer)
		if err != nil {
			return "", err
		}

		parsedVersions[idx] = ver
	}

	sort.Sort(parsedVersions)
	for idx := len(parsedVersions) - 1; idx >= 0; idx-- {
		if constraints.Check(parsedVersions[idx]) {
			return parsedVersions[idx].String(), nil
		}
	}

	return "", eris.New("no matching version found")
}

var noPreRelConstraintPattern = regexp.MustCompile(`[0-9]+\.[0-9]+\.[0-9]+(?:-)?`)

func GetDependencySnapshot(ctx context.Context, q *queries.DBQuerier, modid string, version string) (DependencySnapshot, error) {
	snapshot := make(DependencySnapshot)
	constraints := make(map[string][]constraintItem)
	queue := []modRequest{{
		modid:   modid,
		version: version,
		source:  "<root>",
		preReq:  make(map[string]string),
	}}

	nblog.Log(ctx).Debug().Msgf("Collecting constraints for %s (%s)", modid, version)
	for len(queue) > 0 {
		modReq := queue[0]
		queue = queue[1:]

		pkgs, err := q.GetPublicPackageDependenciesByModVersion(ctx, modReq.modid, modReq.version)
		if err != nil {
			return nil, err
		}

		for _, pkg := range pkgs {
			var deps queryDependencyList
			err = pkg.JsonbAgg.AssignTo(&deps)
			if err != nil {
				return nil, err
			}

			for _, dep := range deps {
				rawConstraint := dep.Version
				if rawConstraint == "" || rawConstraint == "*" {
					rawConstraint = ">= 0.0.0-0"
				}

				// Make sure all constraints allow prerelease versions
				rawConstraint = noPreRelConstraintPattern.ReplaceAllStringFunc(rawConstraint, func(s string) string {
					if !strings.HasSuffix(s, "-") {
						return s + "-0"
					}
					return s
				})
				con, err := semver.NewConstraint(rawConstraint)
				if err != nil {
					return nil, eris.Wrapf(err, "failed to parse constraint %s for mod %s as dependency on %s", dep.Version, dep.Modid, modReq.modid)
				}

				_, present := constraints[dep.Modid]
				if !present {
					pgModVersions, err := q.GetPublicVersionForMod(ctx, dep.Modid)
					if err != nil {
						return nil, eris.Wrapf(err, "failed to retrieve versions for %s", dep.Modid)
					}

					modVersions := make([]string, len(pgModVersions))
					for idx, ver := range pgModVersions {
						modVersions[idx] = ver.String
					}

					version, err := pickNaiveVersion(ctx, modVersions, con)
					if err != nil {
						return nil, eris.Wrapf(err, "failed to resolve dependency %s (%s)", dep.Modid, con)
					}

					preReqs := make(map[string]string)
					for k, v := range modReq.preReq {
						preReqs[k] = v
					}

					preReqs[dep.Modid] = version

					queue = append(queue, modRequest{
						modid:   dep.Modid,
						version: version,
						source:  modReq.modid,
						preReq:  preReqs,
					})
				}

				constraints[dep.Modid] = append(constraints[dep.Modid], constraintItem{
					source:     modReq.modid,
					constraint: con,
					preReq:     modReq.preReq,
				})
			}
		}
	}

	nblog.Log(ctx).Debug().Msgf("Found %d constraints:", len(constraints))
	for modid, cons := range constraints {
		names := make([]string, len(cons))
		for idx, con := range cons {
			names[idx] = con.String()
		}

		nblog.Log(ctx).Debug().Msgf("  %s %s", modid, strings.Join(names, ", "))
	}

	nblog.Log(ctx).Debug().Msg("Resolving constraints")
	for modid, cons := range constraints {
		versions, err := q.GetPublicVersionForMod(ctx, modid)
		if err != nil {
			return nil, eris.Wrapf(err, "failed to look up versions for mod %s", modid)
		}

		var result *semver.Version
		// Check each version (starting with the latest) against our given constraints until we find one that satisfies
		// them.
		for idx := len(versions) - 1; idx >= 0; idx-- {
			version, err := semver.StrictNewVersion(versions[idx].String)
			if err != nil {
				return nil, eris.Wrapf(err, "failed to parse versions for %s", modid)
			}

			ok := true
			for _, item := range cons {
				res, errs := item.constraint.Validate(version)
				if !res {
					nblog.Log(ctx).Debug().Msgf("%s| %s (%s): %+v", modid, version, item.constraint, errs)
					ok = false
					break
				}
			}
			if ok {
				result = version
				break
			}
		}

		if result == nil {
			conList := make([]string, len(cons))
			for idx, item := range cons {
				conList[idx] = item.String()
			}

			return nil, eris.Errorf("can't satisfy constraints: %s %s, available: %s", modid, strings.Join(conList, ", "), spew.Sdump(versions))
		}

		snapshot[modid] = result.String()
		nblog.Log(ctx).Debug().Msgf("%s -> %s", modid, result.String())
	}

	nblog.Log(ctx).Debug().Msg("Verifying requirement assumptions")
	for modid, cons := range constraints {
		for _, con := range cons {
			for reqMod, reqVersion := range con.preReq {
				if snapshot[reqMod] != reqVersion {
					return nil, eris.Errorf("solution failed an assumption made during gather: %s assumed %s (%s) but got %s", modid, reqMod, reqVersion, snapshot[reqMod])
				}
			}
		}
	}

	nblog.Log(ctx).Debug().Msg("Dependencies successfully resolved")
	return snapshot, nil
}
