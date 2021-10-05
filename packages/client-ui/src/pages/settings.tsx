import { useState, useEffect } from 'react';
import { Button, Card, ControlGroup, FormGroup, Slider, Tag } from '@blueprintjs/core';
import { makeAutoObservable, autorun, runInAction } from 'mobx';
import { fromPromise, IPromiseBasedObservable } from 'mobx-utils';
import { observer } from 'mobx-react-lite';
import { Settings, TaskRequest, HardwareInfoResponse, NullMessage } from '@api/client';
import { FinishedUnaryCall } from '@protobuf-ts/runtime-rpc';
import { GlobalState, useGlobalState } from '../lib/state';
import FormContext from '../elements/form-context';
import { FormCheckbox, FormInputGroup, FormSelect } from '../elements/form-elements';

async function loadSettings(gs: GlobalState, formState: Settings): Promise<void> {
  try {
    const result = await gs.client.getSettings({});
    runInAction(() => {
      Object.assign(formState, result.response);
    });
  } catch (e) {
    console.error(e);
  }
}

async function saveSettings(gs: GlobalState, formState: Settings): Promise<void> {
  try {
    await gs.client.saveSettings(formState);
  } catch (e) {
    console.error(e);
  }
}

interface HardwareSelectProps {
  hardwareInfo: IPromiseBasedObservable<FinishedUnaryCall<NullMessage, HardwareInfoResponse>>;
  infoKey: 'audioDevices' | 'captureDevices' | 'resolutions' | 'voices';
  field: string;
}

const HardwareSelect = observer(function HardwareSelect(
  props: HardwareSelectProps,
): React.ReactElement {
  return (
    <FormSelect name={props.field} fill={true}>
      {props.hardwareInfo.case({
        pending: () => <option>Loading...</option>,
        rejected: () => <option>ERROR</option>,
        fulfilled: ({ response }) => (
          <>
            {response[props.infoKey].map((dev) => (
              <option key={dev}>{dev}</option>
            ))}
          </>
        ),
      })}
    </FormSelect>
  );
});

interface JoystickSelectProps {
  hardwareInfo: IPromiseBasedObservable<FinishedUnaryCall<NullMessage, HardwareInfoResponse>>;
}

const JoystickSelect = observer(function JoystickSelect(
  props: JoystickSelectProps,
): React.ReactElement {
  return (
    <FormSelect name="joystick" fill={true}>
      {props.hardwareInfo.case({
        pending: () => <option>Loading...</option>,
        rejected: () => <option>ERROR</option>,
        fulfilled: ({ response }) => (
          <>
            {response.joysticks.map((joystick) => (
              <option key={joystick.uUID} value={joystick.uUID}>
                {joystick.name}
              </option>
            ))}
          </>
        ),
      })}
    </FormSelect>
  );
});

async function selectLibraryFolder(gs: GlobalState, formState: Settings): Promise<void> {
  try {
    const result = await knOpenFolder('Please select your library folder', formState.libraryPath);
    if (result !== '' && result !== formState.libraryPath) {
      runInAction(() => {
        formState.libraryPath = result;
      });

      void rescanLocalMods(gs);
    }
  } catch (e) {
    console.error(e);
  }
}

async function rescanLocalMods(gs: GlobalState): Promise<void> {
  try {
    const task = gs.tasks.startTask('Scan new library folder...');
    await gs.client.scanLocalMods(TaskRequest.create({ ref: task }));
  } catch (e) {
    console.error(e);
  }
}

export default function SettingsPage(): React.ReactElement {
  const gs = useGlobalState();
  const [formState] = useState<Settings>(() => {
    const defaults = Settings.create();
    makeAutoObservable(defaults);
    return defaults;
  });
  const [hardwareInfo] = useState(() => fromPromise(gs.client.getHardwareInfo({})));

  useEffect(() => {
    void loadSettings(gs, formState);

    return autorun(() => {
      // Wait until the settings are loaded; firstRunDone should always be true so it's a good indicator
      if (formState.firstRunDone) {
        console.log('Settings changed...', JSON.stringify(formState));
        void saveSettings(gs, formState);
      }
    });
  }, [gs, formState]);

  return (
    <div className="text-white text-sm">
      <FormContext value={formState as unknown as Record<string, unknown>}>
        <div className="flex flex-row gap-4">
          <div className="flex flex-1 flex-col gap-4">
            <Card>
              <h5 className="text-xl mb-5">General Knossos Settings</h5>
              <FormGroup label="Library Path">
                <ControlGroup fill={true}>
                  <FormInputGroup name="libraryPath" readOnly={true} />
                  <Button
                    onClick={() => {
                      void selectLibraryFolder(gs, formState);
                    }}
                  >
                    Browse...
                  </Button>
                </ControlGroup>
                <Button
                  onClick={() => {
                    void rescanLocalMods(gs);
                  }}
                >
                  Rescan local mods
                </Button>
              </FormGroup>
              <FormCheckbox name="updateCheck" label="Update Notifications" />
              <FormCheckbox name="errorReports" label="Send Error Reports" />
            </Card>
            <Card>
              <h5 className="text-xl mb-5">Downloads</h5>
              <div className="flex flex-row gap-4">
                <FormGroup className="flex-1" label="Max Downloads">
                  <FormInputGroup name="maxDownloads" />
                </FormGroup>

                <FormGroup className="flex-1" label="Download bandwidth limit">
                  <FormInputGroup name="bandwidthLimit" rightElement={<Tag minimal={true}>KiB/s</Tag>} />
                </FormGroup>
              </div>
            </Card>

            <Card>
              <h5 className="text-xl mb-5">Video</h5>

              <div className="flex flex-row gap-4">
                <FormGroup label="Resolution">
                  <HardwareSelect
                    hardwareInfo={hardwareInfo}
                    infoKey="resolutions"
                    field="resolution"
                  />
                </FormGroup>

                <FormGroup label="Bit Depth">
                  <FormSelect name="depth">
                    <option value="32">32-bit</option>
                    <option value="16">16-bit</option>
                  </FormSelect>
                </FormGroup>

                <FormGroup label="Texture Filter">
                  <FormSelect name="filter">
                    <option value="3">Trilinear</option>
                    <option value="2">Bilinear</option>
                  </FormSelect>
                </FormGroup>
              </div>
            </Card>
          </div>
          <div className="flex flex-1 flex-col gap-4">
            <Card>
              <h5 className="text-xl mb-5">Audio</h5>
              <FormGroup label="Playback Device">
                <HardwareSelect
                  hardwareInfo={hardwareInfo}
                  infoKey="audioDevices"
                  field="playback"
                />
              </FormGroup>

              <FormGroup label="Capture Device">
                <HardwareSelect
                  hardwareInfo={hardwareInfo}
                  infoKey="captureDevices"
                  field="capture"
                />
              </FormGroup>

              <FormCheckbox name="efx" label="Enable EFX" />

              <div className="flex flex-row gap-4">
                <FormGroup className="flex-1" label="Sample Rate">
                  <FormInputGroup name="sampleRate" />
                </FormGroup>

                <FormGroup className="flex-1" label="Language">
                  <FormSelect name="language">
                    <option>English</option>
                  </FormSelect>
                </FormGroup>
              </div>
            </Card>

            <Card>
              <h5 className="text-xl mb-5">Speech</h5>
              <div className="flex flex-row gap-4">
                <div className="flex-1">
                  <FormGroup label="Voice">
                    <HardwareSelect hardwareInfo={hardwareInfo} infoKey="voices" field="voice" />
                  </FormGroup>

                  <FormGroup label="Volume">
                    <Slider min={0} max={100} labelStepSize={10} />
                  </FormGroup>
                </div>

                <FormGroup className="flex-1" label="Use Speech in">
                  <FormCheckbox name="speechTechRoom" label="Tech Room" />
                  <FormCheckbox name="speechInGame" label="In-Game" />
                  <FormCheckbox name="speechBriefings" label="Briefings" />
                  <FormCheckbox name="speechMulti" label="Multiplayer" />
                </FormGroup>
              </div>
            </Card>

            <Card>
              <h5 className="text-xl mb-5">Joystick</h5>
              <FormGroup label="Joystick">
                <JoystickSelect hardwareInfo={hardwareInfo} />
              </FormGroup>

              <FormCheckbox name="enableFF" label="Force Feedback" />
              <FormCheckbox name="enableHitEffect" label="Directional Hit" />
            </Card>
          </div>
        </div>
      </FormContext>
    </div>
  );
}
