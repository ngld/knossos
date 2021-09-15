import { useState } from 'react';
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
import { useGlobalState, GlobalState } from '../lib/state';

class WizardState {
  libraryPath: string = '';

  constructor() {
    makeAutoObservable(this);
  }
}

interface LinkProps {
  href: string;
  children: React.ReactNode | React.ReactNode[];
}

function linkHandler(e: React.MouseEvent, gs: GlobalState, link: string): void {
  e.preventDefault();
  void gs.client.openLink({ link });
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

const selectLibraryFolder = action(async function selectLibraryFolder(
  gs: GlobalState,
  state: WizardState,
): Promise<void> {
  try {
    const result = await knOpenFolder('Please select your library folder', state.libraryPath);
    if (result !== '') {
      const rpcResult = await gs.client.fixLibraryFolderPath({ path: result });
      state.libraryPath = rpcResult.response.path;
    }
  } catch (e) {
    console.error(e);
  }
});

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

const retailHandler = action(async function retailHandler(gs: GlobalState, op: string, flavor: string): Promise<void> {
  const ref = gs.tasks.startTask(op === 'unpack' ? 'Unpacking retail files' : 'Copying retail files');
});

const RetailPanel = observer(function RetailPanel(props: StepProps): React.ReactElement {
  const gs = useGlobalState();
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
          <Button onClick={() => retailHandler(gs, 'unpack', 'gog')}>Detect GOG installation</Button>
          {' '}
          <Button onClick={() => retailHandler(gs, 'unpack', 'steam')}>Detect Steam installation</Button>
        </p>
        <p>
          <Button onClick={() => retailHandler(gs, 'unpack', 'inno')}>Unpack GOG installer</Button>
        </p>
        <p>
          <Button onClick={() => retailHandler(gs, 'copy', '')}>Manually select FS2 folder</Button>
        </p>
      </div>
    </div>
  );
});

export interface FirstRunWizardProps {
  onFinished?: () => void;
}

export function FirstRunWizard(props: FirstRunWizardProps): React.ReactElement {
  const [isOpen, setOpen] = useState(true);
  const [state] = useState<WizardState>(() => new WizardState());

  return (
    <MultistepDialog
      className="bp3-ui-text large-dialog"
      title="First Run"
      canOutsideClickClose={true}
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
      />
      <DialogStep
        id="handleRetailFiles"
        title="Retail Files"
        panel={<RetailPanel state={state} />}
      />
    </MultistepDialog>
  );
}
