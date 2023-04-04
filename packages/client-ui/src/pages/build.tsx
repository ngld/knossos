import { useState, useEffect, useCallback, useRef } from 'react';
import {
  Classes,
  Tree,
  type TreeNodeInfo,
  Spinner,
  Button,
  Divider,
  Menu,
  MenuItem,
  MenuDivider,
} from '@blueprintjs/core';
import { SimpleModListResponse_ModInfo } from '@api/client';
import { ContextMenu2 } from '@blueprintjs/popover2';
import { ModMeta, Package } from '@api/mod';
import { gs, OverlayProps } from '../lib/state';
import produce from 'immer';
import ModForm from './build/mod-form';
import PackageForm from './build/package-form';
import CreateModDialog from './build/create-mod-dialog';

type TreeNode = TreeNodeInfo<SimpleModListResponse_ModInfo | Package>;

async function loadPackages(
  setModTree: (cb: (old: TreeNode[]) => TreeNode[]) => void,
  setError: (msg: string) => void,
  idx: number,
  modid: string,
  version: string,
): Promise<void> {
  let result;
  try {
    result = await gs.client.getBuildModRelInfo({ id: modid, version });
  } catch (e) {
    setError(String(e));
    return;
  }

  const pkgs = result.response.packages;

  setModTree(
    produce((tree) => {
      const p = tree[idx];
      p.childNodes = pkgs.map((pkg) => ({
        id: `${modid}-${version}-${pkg.name}`,
        label: (
          <ContextMenu2
            content={
              <Menu>
                <MenuItem text="Open Folder" />
                <MenuDivider />
                <MenuItem text="Delete" icon="delete" intent="danger" />
              </Menu>
            }
          >
            {pkg.name}
          </ContextMenu2>
        ),
        icon: 'box',
        nodeData: pkg,
      }));
    }),
  );
}

export default function BuildPage(): React.ReactElement {
  const [modTree, setModTree] = useState<TreeNode[]>([]);
  const [selectedMod, setSelectedMod] = useState<ModMeta | null>(null);
  const [selectedPkg, setSelectedPkg] = useState<Package | null>(null);
  const [error, setError] = useState<string | null>(null);
  const switchHandler = useRef<() => Promise<boolean>>(async () => true);

  useEffect(() => {
    void (async () => {
      let response;
      try {
        response = await gs.client.getSimpleModList({});
      } catch (e) {
        setError(String(e));
        return;
      }

      setModTree(
        response.response.mods.map((m) => ({
          id: `${m.modid}-${m.version}`,
          label: (
            <ContextMenu2
              content={
                <Menu>
                  <MenuItem text="Launch mod" />
                  <MenuDivider />
                  <MenuItem text="Create Package" icon="add" />
                  <MenuItem text="Delete" icon="delete" intent="danger" />
                </Menu>
              }
            >
              {m.title} - {m.version}
            </ContextMenu2>
          ),
          childNodes: [],
          icon: 'cube',
          nodeData: m,
        })),
      );
    })();
  }, []);

  const expandNode = useCallback(
    (node: TreeNode, nodePath: number[], _e: React.MouseEvent<HTMLElement>) => {
      if (nodePath.length === 1) {
        setModTree(
          produce((tree) => {
            const p = tree[nodePath[0]];
            p.isExpanded = true;
            p.childNodes = [
              {
                id: `${p.id}-loading`,
                label: 'Loading...',
                icon: <Spinner size={15} />,
              },
            ];
          }),
        );

        const modInfo = node.nodeData as SimpleModListResponse_ModInfo;
        void loadPackages(setModTree, setError, nodePath[0], modInfo.modid, modInfo.version);
      }
    },
    [setModTree, setError],
  );

  const collapseNode = useCallback(
    (_node: TreeNode, nodePath: number[], _e: React.MouseEvent<HTMLElement>) => {
      if (nodePath.length === 1) {
        setModTree(
          produce((tree) => {
            tree[nodePath[0]].isExpanded = false;
          }),
        );
      }
    },
    [setModTree],
  );

  const selectNode = useCallback(
    async (node: TreeNode, nodePath: number[], _e: React.MouseEvent<HTMLElement>) => {
      if (!(await switchHandler.current())) {
        return;
      }

      setModTree(
        produce((tree) => {
          for (const modNode of tree) {
            if (modNode.isSelected) {
              modNode.isSelected = false;
            }

            for (const pkgNode of modNode.childNodes ?? []) {
              if (pkgNode.isSelected) {
                pkgNode.isSelected = false;
              }
            }
          }
        }),
      );
      if (nodePath.length === 1) {
        setModTree(
          produce((tree) => {
            tree[nodePath[0]].isSelected = true;
          }),
        );

        const modInfo = node.nodeData as SimpleModListResponse_ModInfo;
        void (async () => {
          try {
            const result = await gs.client.getModInfo({
              id: modInfo.modid,
              version: modInfo.version,
            });
            setSelectedMod(result.response.mod ?? null);
            setSelectedPkg(null);
          } catch (e) {
            setError(String(e));
          }
        })();
      } else if (nodePath.length === 2) {
        setModTree(
          produce((tree) => {
            const p = tree[nodePath[0]];
            if (!p.childNodes) return;

            p.childNodes[nodePath[1]].isSelected = true;

            const pkgInfo = node.nodeData as Package;
            setSelectedMod(null);
            setSelectedPkg(pkgInfo);
          }),
        );
      }
    },
    [setModTree, setError],
  );

  return (
    <div className="text-white flex">
      <div className="flex-initial w-[350px] pr-4">
        <Button
          icon="add"
          onClick={(e) => {
            e.preventDefault();
            gs.launchOverlay(CreateModDialog, {});
          }}
        >
          Create new mod
        </Button>
        <Divider />
        <Tree
          contents={modTree}
          onNodeExpand={expandNode}
          onNodeCollapse={collapseNode}
          onNodeClick={selectNode}
        />
      </div>
      <div className="flex-1">
        {selectedMod ? (
          <ModForm mod={selectedMod} switchHandler={switchHandler} />
        ) : selectedPkg ? (
          <PackageForm pkg={selectedPkg} switchHandler={switchHandler} />
        ) : (
          <p>Please select a mod or package on the left.</p>
        )}
      </div>
    </div>
  );
}
