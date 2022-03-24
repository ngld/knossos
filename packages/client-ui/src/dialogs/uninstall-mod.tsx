import { useState } from 'react';
import { Dialog, Callout, Button, Spinner, Checkbox, Classes } from '@blueprintjs/core';
import { observer } from 'mobx-react-lite';
import { fromPromise } from 'mobx-utils';
import { useGlobalState } from '../lib/state';

interface UninstallModDialogProps {
  modid: string;
  version: string;
  onFinished?: () => void;
}
export default observer(function UninstallModDialog(
  props: UninstallModDialogProps,
): React.ReactElement {
  const [isOpen, setOpen] = useState(true);
  const gs = useGlobalState();
  const [modInfo] = useState(() =>
    fromPromise(gs.client.uninstallModCheck({ modid: props.modid })),
  );
  const [checkedVersions, setCheckedVersions] = useState({ [props.version]: true });

  function triggerUninstall() {
    if (modInfo.state !== 'fulfilled') {
      return;
    }

    const versions: string[] = [];
    const info = modInfo.value.response;

    for (const [version, checked] of Object.entries(checkedVersions)) {
      if (checked && !info.errors[version]) {
        versions.push(version);
      }
    }

    const ref = gs.tasks.startTask('Uninstall mod', () => gs.sendSignal('reloadLocalMods'));
    void gs.client.uninstallMod({ modid: props.modid, versions, ref });
    gs.sendSignal('showTasks');
    setOpen(false);
  }

  let anyValidVersions = false;
  return (
    <Dialog
      className="bp3-ui-text"
      isOpen={isOpen}
      title="Uninstall mod"
      onClose={() => setOpen(false)}
      onClosed={() => {
        if (props.onFinished) {
          props.onFinished();
        }
      }}
    >
      <div className={Classes.DIALOG_BODY}>
        <p className="mb-4">Please select the versions you want to uninstall:</p>
        {modInfo.case({
          pending: () => <Spinner />,
          rejected: (e) => (
            <Callout intent="danger" title="Error">
              <pre>{e instanceof Error ? e.message : String(e)}</pre>
            </Callout>
          ),
          fulfilled: ({ response }) => {
            for (const version of response.versions) {
              if (checkedVersions[version] && !response.errors[version]) {
                anyValidVersions = true;
                break;
              }
            }

            return (
              <ul>
                {response.versions.map((version) => (
                  <li>
                    <Checkbox
                      checked={!!checkedVersions[version] && !response.errors[version]}
                      disabled={!!response.errors[version]}
                      onChange={(e) =>
                        setCheckedVersions((versions) => ({
                          ...versions,
                          [version]: (e.target as HTMLInputElement).checked,
                        }))
                      }
                    >
                      {version}
                    </Checkbox>
                    {response.errors[version] ? (
                      <>
                        <Callout className="mb-4">{response.errors[version]}</Callout>
                      </>
                    ) : null}
                  </li>
                ))}
              </ul>
            );
          },
        })}
      </div>
      <div className={Classes.DIALOG_FOOTER}>
        <div className={Classes.DIALOG_FOOTER_ACTIONS}>
          <Button intent="primary" disabled={!anyValidVersions} onClick={triggerUninstall}>
            Uninstall selected versions
          </Button>
          <Button onClick={() => setOpen(false)}>Close</Button>
        </div>
      </div>
    </Dialog>
  );
});
