import { useState } from 'react';
import { Classes, Button, Dialog } from '@blueprintjs/core';
import { HandleRetailFilesRequest_Operation } from '@api/client';
import { gs, OverlayProps } from '../lib/state';

async function retailHandler(op: HandleRetailFilesRequest_Operation): Promise<void> {
  const ops = HandleRetailFilesRequest_Operation;
  let installerPath = '';

  if (op === ops.MANUAL_FOLDER) {
    try {
      installerPath = await knOpenFolder('Please select your FS2 folder', '');

      if (installerPath === '') {
        return;
      }
    } catch (e) {
      console.error('Dialog failed', e);
      return;
    }
  } else if (op === ops.MANUAL_GOG) {
    try {
      installerPath = await knOpenFile(
        'Please select your FS2 installer (should be setup_freespace_2_1.20_v2_(33372).exe)',
        'setup_freespace_2_1.20_v2_(33372).exe',
        ['Executable|.exe'],
      );

      if (installerPath === '') {
        return;
      }
    } catch (e) {
      console.error('Dialog failed', e);
      return;
    }
  }

  const ref = gs.tasks.startTask('Unpacking retail files and installing FSO');
  gs.sendSignal('showTasks');

  try {
    const resp = await gs.client.getSettings({});
    await gs.client.handleRetailFiles({
      ref,
      op,
      installerPath,
      libraryPath: resp.response.libraryPath,
    });
  } catch (e) {
    console.error('Task failed', e);
  }
}

function RetailPanel(props: OverlayProps): React.ReactElement {
  const [open, setOpen] = useState(true);
  const ops = HandleRetailFilesRequest_Operation;
  return (
    <Dialog
      title="Install Retail Files"
      isOpen={open}
      onClose={() => setOpen(false)}
      onClosed={props.onFinished}
    >
      <div className={Classes.DIALOG_BODY}>
        <div className={Classes.RUNNING_TEXT}>
          <p>
            <Button
              onClick={() => {
                void retailHandler(ops.AUTO_GOG);
                setOpen(false);
              }}
            >
              Detect GOG installation
            </Button>{' '}
            <Button
              onClick={() => {
                void retailHandler(ops.AUTO_STEAM);
                setOpen(false);
              }}
            >
              Detect Steam installation
            </Button>
          </p>
          <p>
            <Button
              onClick={() => {
                void retailHandler(ops.MANUAL_GOG);
                setOpen(false);
              }}
            >
              Unpack GOG installer
            </Button>
          </p>
          <p>
            <Button
              onClick={() => {
                void retailHandler(ops.MANUAL_FOLDER);
                setOpen(false);
              }}
            >
              Manually select FS2 folder
            </Button>
          </p>
        </div>
      </div>
    </Dialog>
  );
}

export default function ToolsPage(): React.ReactElement {
  return (
    <div className="text-white">
      <Button onClick={() => gs.launchOverlay(RetailPanel, {})}>Install Retail Files</Button>
    </div>
  );
}
