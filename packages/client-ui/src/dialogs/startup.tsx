import { useState, useEffect } from 'react';
import { Dialog, Classes, ProgressBar, Callout, Button } from '@blueprintjs/core';
import { runInAction } from 'mobx';
import { GlobalState, useGlobalState } from '../lib/state';
import { runUpdateCheck } from './updater';
import { FirstRunWizard } from './first-run-wizard';
import ErrorDialog from './error-dialog';

export function initStartup(gs: GlobalState): void {
  gs.launchOverlay<StartupOverlayProps>(StartupOverlay, {});
}

interface StartupOverlayProps {
  onFinished?: () => void;
}

function StartupOverlay(props: StartupOverlayProps): React.ReactElement {
  const gs = useGlobalState();
  const [isOpen, setOpen] = useState(true);
  const [label, setLabel] = useState('Launching...');

  useEffect(() => void initSequence(gs, setOpen, setLabel), [gs]);

  return (
    <Dialog
      className="bp3-ui-text"
      isOpen={isOpen}
      onClose={() => setOpen(false)}
      onClosed={() => {
        if (props.onFinished) {
          props.onFinished();
        }
      }}
    >
      <div className={Classes.DIALOG_BODY}>
        <div className="text-lg text-white">{label}</div>
        <ProgressBar intent="primary" stripes={true} animate={true} value={1} />
      </div>
    </Dialog>
  );
}

async function initSequence(
  gs: GlobalState,
  setOpen: React.Dispatch<React.SetStateAction<boolean>>,
  setLabel: React.Dispatch<React.SetStateAction<string>>,
): Promise<void> {
  try {
    setLabel('Loading settings');

    const r = await gs.client.getSettings({});
    if (!r.response.firstRunDone) {
      const idx = gs.launchOverlay(FirstRunWizard, {
        onFinished() {
          gs.removeOverlay(idx);
          void initSequence(gs, setOpen, setLabel);
        },
      });
      setOpen(false);
      return;
    }

    if (r.response.updateCheck) {
      void runUpdateCheck(gs);
    }

    setLabel('Loading installed mods');
    let success = await gs.tasks.runTask('Load local mods', (ref) => {
      gs.sendSignal('showTasks');
      void gs.client.updateLocalModList({ ref });
    });

    if (!success) {
      console.error('Local mod update failed!');
      setOpen(false);
      gs.sendSignal('hideTasks');
      gs.launchOverlay(LocalModLoadErrorOverlay, {folder: r.response.libraryPath});
      return;
    }

    setLabel('Updating mod list');
    success = await gs.tasks.runTask('Sync mods', (ref) => {
      void gs.client.syncRemoteMods({ ref });
    });

    if (!success) {
      console.error('Modsync failed!');
      setOpen(false);
      gs.sendSignal('hideTasks');

      runInAction(() => {
        gs.startupDone = true;
        gs.launchOverlay(ErrorDialog, {
          title: 'Failed to fetch available mods',
          message:
            "Failed to communicate with Nebula, make sure your AV isn't blocking Knossos and Nebula isn't down for maintenance.",
        });
      });
      return;
    }

    gs.sendSignal('hideTasks');
  } catch (e) {
    console.error('Init failed!', e);
    // TODO: user-visible error
  }

  setOpen(false);
  runInAction(() => {
    gs.startupDone = true;
  });
}


function LocalModLoadErrorOverlay(props: {onFinished?: () => void, folder: string}): React.ReactElement {
  const gs = useGlobalState();
  const [isOpen, setOpen] = useState(true);

  return (
    <Dialog
      className="bp3-ui-text"
      isOpen={isOpen}
      onClose={() => setOpen(false)}
      onClosed={() => {
        if (props.onFinished) {
          props.onFinished();
        }
      }}
    >
      <div className={Classes.DIALOG_BODY}>
        <Callout className="overflow-auto" intent="danger" title="Failed to load installed mods">
          <p>Failed to load local mods. Most likely, the mod folder doesn't exist or some issue is preventing Knossos from accessing it.</p>
          <p>If the folder <code>{props.folder}</code> exists, make sure that no anti virus or other program is blocking access to the folder and restart Knossos.</p>
          <p>If the folder doesn't exist, click the button below to set up a new mod folder.</p>
        </Callout>
      </div>
      <div className={Classes.DIALOG_FOOTER}>
        <div className={Classes.DIALOG_FOOTER_ACTIONS}>
          <Button intent="primary" onClick={() => {
            const idx = gs.launchOverlay(FirstRunWizard, {
              onFinished() {
                gs.removeOverlay(idx);
                gs.launchOverlay(StartupOverlay, {});
              },
            });
          }}>
            Start over
          </Button>
        </div>
      </div>
    </Dialog>
  );
}
