import { useState, useEffect } from 'react';
import { observer } from 'mobx-react-lite';
import styled from 'astroturf/react';
import {
  Classes,
  Button,
  Dialog,
  Callout,
  Spinner,
  Tree,
  TreeNodeInfo,
  Checkbox,
  Menu,
  MenuItem,
} from '@blueprintjs/core';
import { ContextMenu2 } from '@blueprintjs/popover2';
import { PackageType } from '@api/mod';
import { InstallInfoResponse_Package, InstallModRequest_Mod } from '@api/client';
import { GlobalState, useGlobalState } from '../lib/state';

interface NodeData {
  modid: string;
  package: InstallInfoResponse_Package;
  selected: boolean;
}

const StyledTree = styled(Tree)`
  /* make packages a bit smaller to save space and disable backgrounds to
     avoid an ugly clash of colors with the checkbox */
  :global(.bp3-tree-node-content-1) {
    height: 25px;

    &:hover {
      background: transparent;
    }
  }

  :global(.bp3-control) {
    margin-bottom: 0;
  }
`;

const NoteBox = styled(Callout).attrs({
  className: 'my-2',
})`
  min-height: 100px;
`;

interface InstallState {
  loading: boolean;
  error: boolean;
  userSelected: Record<string, boolean>;
  nodes: TreeNodeInfo<NodeData>[];
  title: string;
  notes: string;
  modVersions: Record<string, string>;
}

async function getInstallInfo(
  gs: GlobalState,
  props: InstallModDialogProps,
  setState: React.Dispatch<React.SetStateAction<InstallState>>,
): Promise<void> {
  const result = await gs.client.getModInstallInfo({
    id: props.modid ?? '',
    version: props.version ?? '',
  });

  const nodes = [] as TreeNodeInfo<NodeData>[];
  const userSelected = {} as Record<string, boolean>;
  const modVersions = {} as Record<string, string>;
  for (const mod of result.response.mods) {
    modVersions[mod.id] = mod.version;
    nodes.push({
      id: mod.title,
      hasCaret: true,
      isExpanded: mod.id === props.modid,
      icon: 'folder-open',
      label: mod.title,
      secondaryLabel: mod.version,
      childNodes: mod.packages.map((pkg) => {
        const required = pkg.type === PackageType.REQUIRED;
        const selected = required || pkg.type === PackageType.RECOMMENDED;
        userSelected[mod.id + '#' + pkg.name] = selected;

        return {
          id: mod.title + '-' + pkg.name,
          label: '',
          nodeData: {
            modid: mod.id,
            package: pkg,
            selected,
          },
        };
      }),
    });
  }

  processDependencies(userSelected, nodes, setState);
  setState({
    loading: false,
    error: false,
    nodes,
    title: result.response.title,
    notes: '',
    userSelected,
    modVersions,
  });
}

function processDependencies(
  userSelected: Record<string, boolean>,
  tree: TreeNodeInfo<NodeData>[],
  setState: React.Dispatch<React.SetStateAction<InstallState>>,
): void {
  const needed = { ...userSelected };

  // process dependencies for user selection
  for (const mod of tree) {
    for (const node of mod.childNodes ?? []) {
      if (!node.nodeData) {
        continue;
      }
      const pkg = node.nodeData.package;

      if (userSelected[node.nodeData.modid + '#' + pkg.name]) {
        for (const dep of pkg.dependencies) {
          needed[dep.id + '#' + dep.package] = true;
        }

        node.nodeData.selected = true;
      } else {
        node.nodeData.selected = false;
      }
    }
  }

  // make sure all required packages are selected and process dependencies for
  // any packages that had to be selected during this step
  let changes = true;
  while (changes) {
    changes = false;

    for (const mod of tree) {
      for (const node of mod.childNodes ?? []) {
        if (!node.nodeData || node.nodeData.selected) {
          continue;
        }
        const pkg = node.nodeData.package;

        if (needed[node.nodeData.modid + '#' + pkg.name]) {
          node.nodeData.selected = true;
          changes = true;

          for (const dep of pkg.dependencies) {
            needed[dep.id + '#' + dep.package] = true;
          }
        }
      }
    }
  }

  // update the package labels
  for (const mod of tree) {
    for (const node of mod.childNodes ?? []) {
      if (!node.nodeData) {
        continue;
      }
      const pkg = node.nodeData.package;
      const required = pkg.type === PackageType.REQUIRED;

      node.label = (
        <Checkbox
          checked={node.nodeData.selected}
          disabled={required}
          onClick={() => {
            const selID = `${node.nodeData?.modid ?? '_missingID'}#${pkg.name}`;
            userSelected[selID] = !userSelected[selID];
            processDependencies(userSelected, tree, setState);
            setState((prev) => ({ ...prev, userSelected, nodes: tree }));
          }}
        >
          {pkg.name}
        </Checkbox>
      );
    }
  }
}

interface InstallModDialogProps {
  modid?: string;
  version?: string;
  onFinished?: () => void;
}

export const InstallModDialog = observer(function InstallModDialog(props: InstallModDialogProps): React.ReactElement {
  const gs = useGlobalState();
  const [isOpen, setOpen] = useState(true);
  const [state, setState] = useState<InstallState>({
    loading: true,
    error: false,
    userSelected: {},
    modVersions: {},
    nodes: [],
    title: '',
    notes: '',
  });
  useEffect(() => {
    void getInstallInfo(gs, props, setState);
  }, [gs, props]);

  return (
    <Dialog
      className="bp3-ui-text"
      title={'Install ' + state.title}
      isOpen={isOpen}
      onClose={() => setOpen(false)}
      onClosed={() => {
        if (props.onFinished) {
          props.onFinished();
        }
      }}
    >
      <div className={Classes.DIALOG_BODY}>
        {state.loading ? (
          <>
            <div className="text-lg text-white">Fetching data...</div>
            <Spinner />
          </>
        ) : !state.error ? (
          <>
            <ContextMenu2
              content={
                <Menu>
                  <MenuItem
                    text="Expand All"
                    onClick={() => {
                      const newTree = [...state.nodes];
                      for (const root of newTree) {
                        root.isExpanded = true;
                      }
                      setState((prev) => ({ ...prev, nodes: newTree }));
                    }}
                  />
                  <MenuItem
                    text="Collapse All"
                    onClick={() => {
                      const newTree = [...state.nodes];
                      for (const root of newTree) {
                        root.isExpanded = false;
                      }
                      setState((prev) => ({ ...prev, nodes: newTree }));
                    }}
                  />
                </Menu>
              }
            >
              <StyledTree
                contents={state.nodes}
                onNodeExpand={(_node, path) => {
                  const newTree = [...state.nodes];
                  Tree.nodeFromPath(path, newTree).isExpanded = true;
                  setState((prev) => ({ ...prev, nodes: newTree }));
                }}
                onNodeCollapse={(_node, path) => {
                  const newTree = [...state.nodes];
                  Tree.nodeFromPath(path, newTree).isExpanded = false;
                  setState((prev) => ({ ...prev, nodes: newTree }));
                }}
                onNodeClick={(_node, path) => {
                  const newTree = [...state.nodes];
                  const node = Tree.nodeFromPath(path, newTree);
                  const pkg = node.nodeData?.package;
                  if (!pkg) {
                    node.isExpanded = !node.isExpanded;
                    setState((prev) => ({ ...prev, nodes: newTree }));
                  }
                }}
                onNodeMouseEnter={(_node, path) => {
                  const node = Tree.nodeFromPath(path, state.nodes);
                  setState((prev) => ({ ...prev, notes: node.nodeData?.package?.notes ?? '' }));
                }}
              />
            </ContextMenu2>
            <NoteBox title="Notes">{state.notes}</NoteBox>
            <div className={Classes.DIALOG_FOOTER}>
              <div className={Classes.DIALOG_FOOTER_ACTIONS}>
                <Button intent="primary" onClick={() => {
                  setOpen(false);
                  triggerModInstallation(gs, state, props);
                }}>Install Mod</Button>
                <Button onClick={() => setOpen(false)}>Cancel</Button>
              </div>
            </div>
          </>
        ) : (
          <Callout intent="danger" title="Failed to fetch data">
            <pre>{'???'}</pre>
          </Callout>
        )}
      </div>
    </Dialog>
  );
});

export function installMod(gs: GlobalState, modid: string, version: string): void {
  gs.launchOverlay(InstallModDialog, { modid, version });
}

function triggerModInstallation(gs: GlobalState, state: InstallState, props: InstallModDialogProps): void {
  const mods = {} as Record<string, InstallModRequest_Mod>;
  for (const [key, selected] of Object.entries(state.userSelected)) {
    if (selected) {
      const [modID, pkgName] = key.split('#', 2);
      if (!mods[modID]) {
        mods[modID] = {
          modid: modID,
          version: state.modVersions[modID],
          packages: [],
        };
      }

      mods[modID].packages.push(pkgName);
    }
  }

  void gs.client.installMod({
    modid: props.modid ?? '',
    version: props.version ?? '',
    ref: gs.tasks.startTask('Installing mod ' + state.title),
    mods: Object.values(mods),
  });
  gs.sendSignal('showTasks');
}
