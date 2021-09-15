import { useState, useEffect } from 'react';
import { Dialog, Classes, ProgressBar } from '@blueprintjs/core';
import { GlobalState, useGlobalState } from '../lib/state';
import { runUpdateCheck } from './updater';
import { FirstRunWizard, FirstRunWizardProps } from './first-run-wizard';

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

  useEffect(() => void initSequence(gs, setOpen, setLabel), []);

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
    setLabel('Loading settings')

    const r = await gs.client.getSettings({});
    if (!r.response.firstRunDone) {
      gs.launchOverlay<FirstRunWizardProps>(FirstRunWizard, {});
      return;
    }

    if (r.response.updateCheck) {
      void runUpdateCheck(gs);
    }

    setLabel('Loading installed mods');
    // TODO: check mod.json files and mod folders
  } catch (e) {
    console.error('Init failed!', e);
    // TODO: user-visible error
  }

  setOpen(false);
}
