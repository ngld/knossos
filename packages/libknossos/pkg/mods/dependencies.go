package mods

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/rotisserie/eris"

	"github.com/ngld/knossos/packages/api/common"
	"github.com/ngld/knossos/packages/libknossos/pkg/api"
	"github.com/ngld/knossos/packages/libknossos/pkg/storage"
)

type DependencySnapshot map[string]string

type modConstraint struct {
	constraint *semver.Constraints
	modID      string
}

type resolvePathNode struct {
	versionSnapshot  map[string][]string
	requiredPackages map[string][]string
	presentPackages  map[string]bool
	modID            string
	version          string
	constraints      []modConstraint
}

func makeVersionSnapshot(versions map[string][]string) map[string][]string {
	result := make(map[string][]string)
	for k, list := range versions {
		result[k] = make([]string, len(list))
		copy(result[k], list)
	}

	return result
}

var noPreRelConstraintPattern = regexp.MustCompile(`[>=~]*\s*[0-9]+\.[0-9]+\.[0-9]+(?:-)?`)

func GetDependencySnapshot(ctx context.Context, mods storage.ModProvider, release *common.Release) (DependencySnapshot, error) {
	startTime := time.Now()

	availableVersions := make(map[string][]string)
	path := make([]resolvePathNode, 0)
	queue := []string{release.Modid}
	conflicts := make(map[string]map[string]string)

	availableVersions[release.Modid] = []string{release.Version}

	for len(queue) > 0 {
		modID := queue[0]
		queue = queue[1:]

	repickVersion:
		if len(availableVersions[modID]) < 1 {
			api.Log(ctx, api.LogDebug, "DEP: No versions left to try for %s, redoing previous mod.", modID)

			if len(path) < 2 {
				api.Log(ctx, api.LogDebug, "DEP: Reached root; no mod left to redo, failing!")

				var messages map[string]string
				for _, msgs := range conflicts {
					if len(msgs) > len(messages) {
						messages = msgs
					}
				}

				if messages == nil {
					return nil, eris.New("unable to satisfy constraints")
				}

				msgList := make([]string, 0, len(messages))
				for _, msg := range messages {
					msgList = append(msgList, msg)
				}

				return nil, eris.Errorf("could not resolve conflict: %s which doesn't match what some of the other mods require", strings.Join(msgList, "\n"))
			}

			// Remove the last path mod from the path, put it back in the queue (along with our current mod)
			// restore the snapshot of available versions and remove the previously picked version from the pool.
			lastNode := path[len(path)-1]
			path = path[:len(path)-1]

			queue = append([]string{lastNode.modID, modID}, queue...)
			availableVersions = lastNode.versionSnapshot
			modVersions := availableVersions[lastNode.modID]

			if modVersions[len(modVersions)-1] != lastNode.version {
				panic("consistency error; path version doesn't match expected available version")
			}

			availableVersions[lastNode.modID] = modVersions[:len(modVersions)-1]
			// Let's start processing the mod again...
			continue
		}

		version := availableVersions[modID][len(availableVersions[modID])-1]
		api.Log(ctx, api.LogDebug, "DEP: Trying %s %s", modID, version)

		parsedVersion, err := semver.NewVersion(version)
		if err != nil {
			return nil, eris.Wrapf(err, "failed to parse version %s for mod %s", version, modID)
		}

		rel, err := mods.GetModRelease(ctx, modID, version)
		if err != nil {
			return nil, eris.Wrapf(err, "failed to retrieve mod %s %s", modID, version)
		}

		pkgs := FilterUnsupportedPackages(ctx, rel.Packages)
		if len(pkgs) < 1 {
			api.Log(ctx, api.LogDebug, "DEP: Conflict: %s %s not supported on current platform, picking next version.", modID, version)
			availableVersions[modID] = availableVersions[modID][:len(availableVersions[modID])-1]
			goto repickVersion
		}

		presentPackages := make(map[string]bool)
		for _, pkg := range pkgs {
			presentPackages[pkg.Name] = true
		}

		// Ensure this version is compatible with previously chosen mods.
		for _, node := range path {
			for _, con := range node.constraints {
				if con.modID == modID {
					ok, err := con.constraint.Validate(parsedVersion)
					if !ok {
						api.Log(ctx, api.LogDebug, "DEP: Conflict with %s %s: %s", node.modID, node.version, err)
						availableVersions[modID] = availableVersions[modID][:len(availableVersions[modID])-1]
						goto repickVersion
					}
				}
			}

			if neededPkgs, ok := node.requiredPackages[modID]; ok {
				for _, needed := range neededPkgs {
					if !presentPackages[needed] {
						api.Log(ctx, api.LogDebug, "DEP: Conflict with %s %s: requires missing package %s", node.modID, node.version, needed)
						availableVersions[modID] = availableVersions[modID][:len(availableVersions[modID])-1]
						goto repickVersion
					}
				}
			}
		}

		// Collect constraints
		requiredPackages := make(map[string][]string)
		cons := make([]modConstraint, 0)
		for _, pkg := range pkgs {
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

				constraint, err := semver.NewConstraint(rawConstraint)
				if err != nil {
					return nil, eris.Wrapf(err, "failed to parse constraint %s for mod %s %s", dep.Constraint, modID, version)
				}

				for _, node := range path {
					if node.modID == dep.Modid {
						parsedVersion, err := semver.NewVersion(node.version)
						if err != nil {
							return nil, eris.Wrapf(err, "failed to parse version %s for mod %s during constraint check", node.version, node.modID)
						}

						if !constraint.Check(parsedVersion) {
							api.Log(ctx, api.LogDebug, "DEP: Conflict with %s %s: previously picked version conflicts with constraint %s on package %s", node.modID, node.version, dep.Constraint, pkg.Name)
							availableVersions[modID] = availableVersions[modID][:len(availableVersions[modID])-1]
							goto repickVersion
						}

						for _, reqPkg := range dep.Packages {
							if !node.presentPackages[reqPkg] {
								api.Log(ctx, api.LogDebug, "DEP: Conflict with %s %s: previously picked version is missing package %s required by %s", node.modID, node.version, reqPkg, pkg.Name)
								availableVersions[modID] = availableVersions[modID][:len(availableVersions[modID])-1]
								goto repickVersion
							}
						}
					}
				}

				requiredPackages[dep.Modid] = append(requiredPackages[dep.Modid], dep.Packages...)
				cons = append(cons, modConstraint{
					modID:      dep.Modid,
					constraint: constraint,
				})
			}
		}

		conflictSnapshot := makeVersionSnapshot(availableVersions)
		queueSnapshot := make([]string, len(queue))
		copy(queueSnapshot, queue)

		// Remove all conflicting versions from availableVersions
		for _, con := range cons {
			versions, ok := availableVersions[con.modID]
			if !ok {
				versions, err = mods.GetVersionsForMod(ctx, con.modID)
				if err != nil {
					return nil, eris.Wrapf(err, "failed to fetch versions for mod %s during constraint check", modID)
				}

				availableVersions[con.modID] = versions
				queue = append(queue, con.modID)
			}

			for idx := len(versions) - 1; idx >= 0; idx-- {
				parsedVersion, err := semver.NewVersion(versions[idx])
				if err != nil {
					return nil, eris.Wrapf(err, "failed to parse version %s for mod %s during constraint check", versions[idx], con.modID)
				}

				ok, errs := con.constraint.Validate(parsedVersion)
				if !ok {
					api.Log(ctx, api.LogDebug, "DEP: Removed %s %s due to %s from %s (%s)", con.modID, versions[idx], con.constraint.String(), modID, errs)
					versions = append(versions[:idx], versions[idx+1:]...)
				}
			}

			if len(versions) < 1 {
				api.Log(ctx, api.LogDebug, "DEP: Conflict: no versions left for %s after processing constraints for %s, picking next version.", con.modID, modID)

				// Log conflict
				msgs, ok := conflicts[con.modID]
				if !ok {
					msgs = make(map[string]string)
					conflicts[con.modID] = msgs
				}
				msgs[modID] = fmt.Sprintf("%s requires %s %s which couldn't be fulfilled", modID, con.modID, con.constraint)

				availableVersions = conflictSnapshot
				availableVersions[modID] = availableVersions[modID][:len(availableVersions[modID])-1]
				queue = queueSnapshot
				goto repickVersion
			}

			availableVersions[con.modID] = versions
		}

		path = append(path, resolvePathNode{
			modID:            modID,
			version:          version,
			constraints:      cons,
			versionSnapshot:  makeVersionSnapshot(availableVersions),
			presentPackages:  presentPackages,
			requiredPackages: requiredPackages,
		})
	}

	api.Log(ctx, api.LogDebug, "DEP: Resolve done in %.3fms; building snapshot", float32(time.Since(startTime).Microseconds())/1000)
	snapshot := make(DependencySnapshot)

	for _, node := range path[1:] {
		snapshot[node.modID] = node.version
	}

	return snapshot, nil
}

func GetModDependents(ctx context.Context, mods storage.ModProvider, modID, version string) ([][2]string, error) {
	result := make([][2]string, 0)
	modList, err := mods.GetAllReleases(ctx)
	if err != nil {
		return nil, eris.Wrap(err, "failed to read mod list")
	}

	for _, rel := range modList {
		if rel.Modid == modID {
			continue
		}

		if rel.DependencySnapshot == nil {
			api.Log(ctx, api.LogWarn, "Mod %s %s doesn't have a dependency snapshot, skipping it!", rel.Modid, rel.Version)
			continue
		}

		if rel.DependencySnapshot[modID] == version {
			result = append(result, [2]string{rel.Modid, rel.Version})
		}
	}

	return result, nil
}
