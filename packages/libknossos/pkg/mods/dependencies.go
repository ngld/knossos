package mods

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/rotisserie/eris"

	"github.com/ngld/knossos/packages/api/common"
	"github.com/ngld/knossos/packages/libknossos/pkg/api"
	"github.com/ngld/knossos/packages/libknossos/pkg/storage"
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
	mod    *common.Release
	preReq map[string]string
	source string
}

func (item constraintItem) String() string {
	return fmt.Sprintf("%s (%s)", item.constraint, item.source)
}

func pickNaiveVersion(_ context.Context, available []string, constraints *semver.Constraints) (string, error) {
	parsedVersions := make(semver.Collection, len(available))
	for idx, rawVer := range available {
		ver, err := semver.StrictNewVersion(rawVer)
		if err != nil {
			return "", eris.Wrapf(err, "failed to parse version %s", rawVer)
		}

		parsedVersions[idx] = ver
	}

	sort.Sort(parsedVersions)
	for idx := len(parsedVersions) - 1; idx >= 0; idx-- {
		if constraints.Check(parsedVersions[idx]) {
			return parsedVersions[idx].String(), nil
		}
	}

	return "", eris.Errorf("no matching version found (looking for %s, available %s)", constraints, strings.Join(available, ", "))
}

var noPreRelConstraintPattern = regexp.MustCompile(`[>=~]*\s*[0-9]+\.[0-9]+\.[0-9]+(?:-)?`)

func GetDependencySnapshot(ctx context.Context, mods storage.ModProvider, release *common.Release) (DependencySnapshot, error) {
	snapshot := make(DependencySnapshot)
	constraints := make(map[string][]constraintItem)
	blocklist := make(map[string]bool)
	queue := []modRequest{{
		mod:    release,
		source: "<root>",
		preReq: make(map[string]string),
	}}

	api.Log(ctx, api.LogDebug, "Collecting constraints for %s (%s)", release.Modid, release.Version)
	for len(queue) > 0 {
		modReq := queue[0]
		queue = queue[1:]

		for _, pkg := range modReq.mod.Packages {
			for _, dep := range pkg.Dependencies {
				rawConstraint := dep.Constraint
				if rawConstraint == "" || rawConstraint == "*" {
					rawConstraint = ">= 0.0.0-0"
				}

				// Make sure all constraints that don't require exact versions allow prerelease versions
				rawConstraint = noPreRelConstraintPattern.ReplaceAllStringFunc(rawConstraint, func(s string) string {
					if !strings.HasSuffix(s, "-") && strings.ContainsAny(s, ">~") {
						return s + "-0"
					}
					return s
				})
				con, err := semver.NewConstraint(rawConstraint)
				if err != nil {
					return nil, eris.Wrapf(err, "failed to parse constraint %s for mod %s as dependency on %s", dep.Constraint, dep.Modid, modReq.mod.Modid)
				}

				_, present := constraints[dep.Modid]
				if !present {
					modVersions, err := mods.GetVersionsForMod(ctx, dep.Modid)
					if err != nil {
						return nil, eris.Wrapf(err, "failed to retrieve versions for %s", dep.Modid)
					}

				repickModVersion:
					version, err := pickNaiveVersion(ctx, modVersions, con)
					if err != nil {
						return nil, eris.Wrapf(err, "failed to resolve dependency %s (%s)", dep.Modid, con)
					}

					depRel, err := mods.GetModRelease(ctx, dep.Modid, version)
					if err != nil {
						return nil, eris.Wrapf(err, "failed to fetch metadata for mod %s (%s)", dep.Modid, version)
					}

					depRel.Packages = FilterUnsupportedPackages(ctx, depRel.Packages)
					api.Log(ctx, api.LogDebug, "Found %d pkgs for %s %s", len(depRel.Packages), depRel.Modid, depRel.Version)
					if len(depRel.Packages) == 0 {
						// This release doesn't support the current platform, remove this version and try again
						blocklist[dep.Modid+"#"+version] = true
						for idx, v := range modVersions {
							if v == version {
								modVersions = append(modVersions[:idx], modVersions[idx+1:]...)
								goto repickModVersion
							}
						}

						// pickNaiveVersion picked a version that isn't in modVersions??!
						panic("internal consistency error")
					}

					preReqs := make(map[string]string)
					for k, v := range modReq.preReq {
						preReqs[k] = v
					}

					preReqs[dep.Modid] = version

					queue = append(queue, modRequest{
						mod:    depRel,
						source: modReq.mod.Modid,
						preReq: preReqs,
					})
				}

				constraints[dep.Modid] = append(constraints[dep.Modid], constraintItem{
					source:     modReq.mod.Modid,
					constraint: con,
					preReq:     modReq.preReq,
				})
			}
		}
	}

	api.Log(ctx, api.LogDebug, "Found %d constraints:", len(constraints))
	for modid, cons := range constraints {
		names := make([]string, len(cons))
		for idx, con := range cons {
			names[idx] = con.String()
		}

		api.Log(ctx, api.LogDebug, "  %s %s", modid, strings.Join(names, ", "))
	}

	api.Log(ctx, api.LogDebug, "Resolving constraints")
	for modid, cons := range constraints {
		versions, err := mods.GetVersionsForMod(ctx, modid)
		if err != nil {
			return nil, eris.Wrapf(err, "failed to look up versions for mod %s", modid)
		}

		var result *semver.Version
		// Check each version (starting with the latest) against our given constraints until we find one that satisfies
		// them.
		for idx := len(versions) - 1; idx >= 0; idx-- {
			if blocklist[modid+"#"+versions[idx]] {
				continue
			}

			version, err := semver.StrictNewVersion(versions[idx])
			if err != nil {
				return nil, eris.Wrapf(err, "failed to parse versions for %s", modid)
			}

			ok := true
			for _, item := range cons {
				res, errs := item.constraint.Validate(version)
				if !res {
					api.Log(ctx, api.LogDebug, "%s| %s (%s): %+v", modid, version, item.constraint, errs)
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

			return nil, eris.Errorf("can't satisfy constraints: %s %s, available: %s", modid, strings.Join(conList, ", "), strings.Join(versions, ", "))
		}

		snapshot[modid] = result.String()
		api.Log(ctx, api.LogDebug, "%s -> %s", modid, result.String())
	}

	api.Log(ctx, api.LogDebug, "Verifying requirement assumptions")
	for modid, cons := range constraints {
		for _, con := range cons {
			for reqMod, reqVersion := range con.preReq {
				if snapshot[reqMod] != reqVersion {
					return nil, eris.Errorf("solution failed an assumption made during gather: %s assumed %s (%s) but got %s", modid, reqMod, reqVersion, snapshot[reqMod])
				}
			}
		}
	}

	api.Log(ctx, api.LogDebug, "Dependencies successfully resolved")
	return snapshot, nil
}
