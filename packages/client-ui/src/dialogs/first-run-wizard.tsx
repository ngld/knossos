import { useState, useEffect } from 'react';
import {
  MultistepDialog,
  DialogStep,
  H1,
  Code,
  Classes,
  Button,
  FormGroup,
  ControlGroup,
  InputGroup,
} from '@blueprintjs/core';
import { makeAutoObservable, action, runInAction } from 'mobx';
import { observer } from 'mobx-react-lite';
import { HandleRetailFilesRequest_Operation } from '@api/client';
import { useGlobalState, GlobalState } from '../lib/state';
import ErrorDialog from './error-dialog';

class WizardState {
  libraryPath: string = '';
  retailDone = false;

  constructor() {
    makeAutoObservable(this);
  }
}

interface LinkProps {
  href: string;
  children: React.ReactNode | React.ReactNode[];
}

async function linkHandler(e: React.MouseEvent, gs: GlobalState, link: string): Promise<void> {
  e.preventDefault();
  try {
    await gs.client.openLink({ link });
  } catch (e) {
    gs.launchOverlay(ErrorDialog, {
      title: 'Error',
      message: e instanceof Error ? e.message : String(e),
    });
  }
}

function Link(props: LinkProps): React.ReactElement {
  const gs = useGlobalState();
  return (
    <a href={props.href} onClick={(e) => linkHandler(e, gs, props.href)}>
      {props.children}
    </a>
  );
}

function IntroPanel(): React.ReactElement {
  return (
    <div className={Classes.DIALOG_BODY}>
      <H1>Welcome to Knossos</H1>
      <div className={Classes.RUNNING_TEXT}>
        <p>
          With Knossos you will be able to install and launch mods, total conversions (TC) and
          engine updates for Freespace 2.
        </p>
        <p>There are a few important things I'd like to point out before we get started:</p>
        <ul>
          <li>
            Modding for FreeSpace 2 works a bit differently than you might be used to. You can only
            enable a single mod at any time. Most mods (&gt; 90%) add new content and you will only
            be able to play the content for the selected mod. The launcher <i>can</i> load multiple
            mods at once and will do so most of the time but that functionality is only used to load
            dependencies.
          </li>
          <li>
            If you get stuck at any point and/or need help, we have a{' '}
            <Link href="https://www.hard-light.net/forums/index.php?board=151.0">forum</Link> and a{' '}
            <Link href="https://discord.gg/QFdueKEYrN">Discord</Link>.
          </li>
          <li>
            Whenever you see "retail" mentioned in the context of FS2 modding, it usually refers to
            the files in the retail copy of FS2. This doesn't have to be a physical copy, a digital
            download from GOG or Steam contains the same files.
          </li>
          <li>
            Total conversions recreated all necessary files from scratch which means that you can
            play them if you haven't bought FS2. To play those, you'll just have to skip the FS2
            step in this wizard. Knossos will handle the rest.
          </li>
        </ul>
      </div>
    </div>
  );
}

async function selectLibraryFolder(gs: GlobalState, state: WizardState): Promise<void> {
  try {
    const result = await knOpenFolder('Please select your library folder', state.libraryPath);
    if (result !== '') {
      const rpcResult = await gs.client.fixLibraryFolderPath({ path: result });

      runInAction(() => {
        state.libraryPath = rpcResult.response.path;
      });
    }
  } catch (e) {
    console.error(e);
  }
}

interface StepProps {
  state: WizardState;
}

const SelectLibraryFolder = observer(function SelectLibraryFolder(
  props: StepProps,
): React.ReactElement {
  const gs = useGlobalState();
  return (
    <div className={Classes.DIALOG_BODY}>
      <div className={Classes.RUNNING_TEXT}>
        <p>Please select your library folder.</p>
        <p>
          This folder will contain your installed mods and other files. This folder can grow quite
          large depending on the amount of installed mods (50 GiB are normal) so please put this on
          a drive with enough free space.
        </p>
        <p>
          If you haven't used Knossos before, select a new empty folder. If you're reinstalling
          Knossos or have a backup of your library folder. Select that folder here. It should
          contain a <Code>knossos.library</Code> file.
        </p>
      </div>
      <FormGroup label="Library Path">
        <ControlGroup fill={true}>
          <InputGroup name="libraryPath" readOnly={true} value={props.state.libraryPath} />
          <Button
            onClick={() => {
              void selectLibraryFolder(gs, props.state);
            }}
          >
            Browse...
          </Button>
        </ControlGroup>
      </FormGroup>
    </div>
  );
});

async function retailHandler(
  gs: GlobalState,
  state: WizardState,
  op: HandleRetailFilesRequest_Operation,
): Promise<void> {
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

  const ref = gs.tasks.startTask('Unpacking retail files', (success) => {
    if (success) {
      runInAction(() => {
        state.retailDone = true;
      });
    }
  });
  gs.sendSignal('showTasks');

  try {
    await gs.client.handleRetailFiles({ ref, op, installerPath, libraryPath: state.libraryPath });
  } catch (e) {
    console.error('Task failed', e);
  }
}

const RetailPanel = observer(function RetailPanel(props: StepProps): React.ReactElement {
  const gs = useGlobalState();
  const ops = HandleRetailFilesRequest_Operation;
  return (
    <div className={Classes.DIALOG_BODY}>
      <div className={Classes.RUNNING_TEXT}>
        <p>
          In this step we'll copy the retail files from your existing FS2 installation or the GOG
          offline installer to our library folder.
        </p>
        <p>
          If you only want to play total conversions (TCs; like Diaspora, The Babylon Project or
          Solaris), you can skip this step.
        </p>
        <p>
          <Button onClick={() => retailHandler(gs, props.state, ops.AUTO_GOG)}>
            Detect GOG installation
          </Button>{' '}
          <Button onClick={() => retailHandler(gs, props.state, ops.AUTO_STEAM)}>
            Detect Steam installation
          </Button>
        </p>
        <p>
          <Button onClick={() => retailHandler(gs, props.state, ops.MANUAL_GOG)}>
            Unpack GOG installer
          </Button>
        </p>
        <p>
          <Button onClick={() => retailHandler(gs, props.state, ops.MANUAL_FOLDER)}>
            Manually select FS2 folder
          </Button>
        </p>
      </div>
    </div>
  );
});

const FinalPanel = observer(function FinalPanel(props: StepProps): React.ReactElement {
  const gs = useGlobalState();
  useEffect(() => {
    void (async () => {
      try {
        const result = await gs.client.getSettings({});
        const settings = result.response;

        settings.libraryPath = props.state.libraryPath;
        settings.firstRunDone = true;

        await gs.client.saveSettings(settings);
      } catch (e) {
        console.error('Failed to save library folder', e);
      }
    })();
  }, [props.state.libraryPath]);

  return (
    <div className={Classes.DIALOG_BODY}>
      <div className={Classes.RUNNING_TEXT}>
        <p>Congratulations! You're now ready to use Knossos.</p>
        <p>I just have a few final words of advice:</p>
        <ul>
          {props.state.retailDone ? (
            <li>
              You'll see that one item that looks like a mod is already installed, called "Retail
              FS2". If you launch this "mod", you'll see the original unmodded FS2 game running on
              the new FSO engine. It's generally recommended to install and launch the MediaVPs mod
              instead since that provides updated textures, models and various fixes. You won't get
              the full FSO experience without it.
            </li>
          ) : null}
          <li>
            On the Explore tab, you'll find available mods which you can install. Once installed,
            they'll appear on the Play tab. Clicking the Play button on a mod there will take you
            straight into the game.
          </li>
          <li>
            The Build tab will allow you to create and modify mods but it's not implemented, yet.
          </li>
        </ul>
      </div>
    </div>
  );
});

export interface FirstRunWizardProps {
  onFinished?: () => void;
}

export const FirstRunWizard = observer(function FirstRunWizard(
  props: FirstRunWizardProps,
): React.ReactElement {
  const [isOpen, setOpen] = useState(true);
  const [state] = useState<WizardState>(() => new WizardState());

  return (
    <MultistepDialog
      className="bp3-ui-text large-dialog"
      title="First Run"
      finalButtonProps={{
        text: 'Finish',
        onClick() {
          setOpen(false);
        },
      }}
      canOutsideClickClose={false}
      isCloseButtonShown={false}
      isOpen={isOpen}
      onClose={() => setOpen(false)}
      onClosed={() => {
        if (props.onFinished) {
          props.onFinished();
        }
      }}
    >
      <DialogStep id="intro" title="Intro" panel={<IntroPanel />} />
      <DialogStep
        id="selectLibraryFolder"
        title="Select Library Folder"
        panel={<SelectLibraryFolder state={state} />}
        nextButtonProps={{ disabled: state.libraryPath === '' }}
      />
      <DialogStep
        id="handleRetailFiles"
        title="Retail Files"
        panel={<RetailPanel state={state} />}
        nextButtonProps={{ text: state.retailDone ? 'Next' : 'Skip' }}
      />
      <DialogStep id="finish" title="Finish" panel={<FinalPanel state={state} />} />
    </MultistepDialog>
  );
});
