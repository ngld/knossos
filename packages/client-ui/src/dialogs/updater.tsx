import React, { useState } from 'react';
import { Dialog, Classes, Button } from '@blueprintjs/core';
import { GlobalState, useGlobalState } from '../lib/state';

export async function runUpdateCheck(gs: GlobalState): Promise<void> {
  try {
    const result = await gs.client.checkForProgramUpdates({});
    if (result.response.knossos !== '' || result.response.updater !== '') {
      gs.launchOverlay<UpdaterPromptProps>(UpdaterPrompt, {
        knossosVersion: result.response.knossos,
        updaterVersion: result.response.updater,
      });
    }
  } catch (e) {
    console.error('Failed to check for updates', e);
  }
}

interface UpdaterPromptProps {
  knossosVersion: string;
  updaterVersion: string;
  onFinished?: () => void;
}

function UpdaterPrompt(props: UpdaterPromptProps): React.ReactNode {
  const gs = useGlobalState();
  const [isOpen, setOpen] = useState(true);
  return (
    <Dialog
      className="bp3-ui-text"
      title="Update available"
      isOpen={isOpen}
      onClose={() => setOpen(false)}
      onClosed={() => {
        if (props.onFinished) {
          props.onFinished();
        }
      }}
    >
      <div className={Classes.DIALOG_BODY}>
        {/* an update to the updater always trumps a Knossos update */}
        {props.updaterVersion !== ''
          ? `An update for Knossos' updater is available (new version: ${props.updaterVersion}).`
          : `An update to Knossos ${props.knossosVersion} is available.`}
        <br />
        Install this update now?
        <div className={Classes.DIALOG_FOOTER}>
          <div className={Classes.DIALOG_FOOTER_ACTIONS}>
            <Button
              intent="primary"
              onClick={() => {
                setOpen(false);
                triggerUpdate(gs, props);
              }}
            >
              Yes
            </Button>
            <Button onClick={() => setOpen(false)}>No</Button>
          </div>
        </div>
      </div>
    </Dialog>
  );
}

async function triggerUpdate(gs: GlobalState, props: UpdaterPromptProps): Promise<void> {
  const taskId = gs.tasks.startTask('Installing update', (success) => {
    if (success) {
      if (props.updaterVersion !== '' && props.knossosVersion !== '') {
        // We just finished updating the updater. Now run the prompt for the Knossos update.
        void runUpdateCheck(gs);
      } else if (props.updaterVersion === '') {
        // We launched the updater for Knossos, now close Knossos to let the updater do its thing.
        knCloseWindow();
      }
    }
  });

  if (props.updaterVersion !== '') {
    await gs.client.updateUpdater({ ref: taskId });
  } else if (props.knossosVersion !== '') {
    await gs.client.updateKnossos({ ref: taskId });
  }
}
