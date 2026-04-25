package stack

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"myplatform/internal/runtime"
	"myplatform/internal/services"
	"myplatform/internal/utils"
)

// mergeEnv combines an env list (["KEY=VALUE", ...]) with an environment map
// ({KEY: VALUE, ...}) into a single slice. Map keys are sorted for determinism.
func mergeEnv(list []string, envMap map[string]string) []string {
	if len(envMap) == 0 {
		return list
	}
	keys := make([]string, 0, len(envMap))
	for k := range envMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	merged := make([]string, len(list), len(list)+len(keys))
	copy(merged, list)
	for _, k := range keys {
		merged = append(merged, k+"="+envMap[k])
	}
	return merged
}

// printDeployOrder prints a single info line listing the resolved deploy order.
func printDeployOrder(ordered []NamedService) {
	names := make([]string, len(ordered))
	for i, ns := range ordered {
		names[i] = ns.Key
	}
	utils.PrintInfo("Deployment order: " + strings.Join(names, " → "))
}

// Deploy deploys all services in dependency order.
// Services whose container already exists are skipped with a warning.
func Deploy(ctx context.Context, rt runtime.RuntimeClient, s *Stack) error {
	ordered, err := ResolveOrder(s.Services)
	if err != nil {
		return err
	}

	printDeployOrder(ordered)
	fmt.Println()

	for _, ns := range ordered {
		name := ns.Def.ContainerName

		exists, err := rt.ContainerExists(ctx, name)
		if err != nil {
			return err
		}
		if exists {
			utils.PrintWarning(fmt.Sprintf("[%s] Container %q already exists — skipping", ns.Key, name))
			continue
		}

		for _, vol := range ns.Def.Volumes {
			volName := strings.SplitN(vol, ":", 2)[0]
			utils.PrintInfo(fmt.Sprintf("[%s] Creating volume %q...", ns.Key, volName))
			if err := rt.CreateVolume(ctx, volName); err != nil {
				return err
			}
		}

		utils.PrintInfo(fmt.Sprintf("[%s] Pulling image %q...", ns.Key, ns.Def.Image))
		if err := rt.PullImage(ctx, ns.Def.Image); err != nil {
			return err
		}

		cfg := services.ServiceConfig{
			Name:          ns.Key,
			Image:         ns.Def.Image,
			ContainerName: name,
			Ports:         ns.Def.Ports,
			Volumes:       ns.Def.Volumes,
			Env:           mergeEnv(ns.Def.Env, ns.Def.Environment),
			Command:       ns.Def.Command,
		}
		utils.PrintInfo(fmt.Sprintf("[%s] Starting container %q...", ns.Key, name))
		if err := rt.RunContainer(ctx, cfg); err != nil {
			return err
		}
		utils.PrintSuccess(fmt.Sprintf("[%s] Deployed -> %q", ns.Key, name))
	}
	return nil
}

// Remove stops and removes every service container in reverse dependency order
// so that dependents are torn down before the services they relied on.
// If removeVolumes is true, named volumes declared for each service are also deleted.
// Missing containers are skipped with a warning so a partial stack can be cleaned up.
func Remove(ctx context.Context, rt runtime.RuntimeClient, s *Stack, removeVolumes bool) error {
	ordered, err := ResolveOrder(s.Services)
	if err != nil {
		return err
	}

	// Reverse: teardown dependents first.
	for i, j := 0, len(ordered)-1; i < j; i, j = i+1, j-1 {
		ordered[i], ordered[j] = ordered[j], ordered[i]
	}

	for _, ns := range ordered {
		name := ns.Def.ContainerName

		exists, err := rt.ContainerExists(ctx, name)
		if err != nil {
			return err
		}
		if !exists {
			utils.PrintWarning(fmt.Sprintf("[%s] Container %q does not exist — skipping", ns.Key, name))
			continue
		}

		utils.PrintInfo(fmt.Sprintf("[%s] Stopping %q...", ns.Key, name))
		if err := rt.StopContainer(ctx, name); err != nil {
			return err
		}
		utils.PrintInfo(fmt.Sprintf("[%s] Removing %q...", ns.Key, name))
		if err := rt.RemoveContainer(ctx, name); err != nil {
			return err
		}
		utils.PrintSuccess(fmt.Sprintf("[%s] Removed %q", ns.Key, name))

		if removeVolumes {
			for _, vol := range ns.Def.Volumes {
				volName := strings.SplitN(vol, ":", 2)[0]
				utils.PrintInfo(fmt.Sprintf("[%s] Removing volume %q...", ns.Key, volName))
				if err := rt.RemoveVolume(ctx, volName); err != nil {
					utils.PrintWarning(fmt.Sprintf("[%s] Could not remove volume %q: %v", ns.Key, volName, err))
				} else {
					utils.PrintSuccess(fmt.Sprintf("[%s] Volume %q removed", ns.Key, volName))
				}
			}
		}
	}
	return nil
}

// Status prints a table of runtime state for every service in the stack.
func Status(ctx context.Context, rt runtime.RuntimeClient, s *Stack) error {
	w := utils.NewTabWriter(os.Stdout)
	fmt.Fprintln(w, "SERVICE\tCONTAINER\tSTATE\tPORTS")
	fmt.Fprintln(w, "-------\t---------\t-----\t-----")

	for _, ns := range s.Services {
		name := ns.Def.ContainerName
		exists, err := rt.ContainerExists(ctx, name)
		if err != nil {
			return err
		}
		if !exists {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", ns.Key, name, "not deployed", "-")
			continue
		}

		info, err := rt.InspectContainer(ctx, name)
		if err != nil {
			return err
		}
		ports := info.Ports
		if ports == "" {
			ports = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", ns.Key, name, info.Status, ports)
	}

	return w.Flush()
}
