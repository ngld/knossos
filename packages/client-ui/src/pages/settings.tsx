import { useState } from 'react';
import { Button, Card, ControlGroup, FormGroup, Tag, Spinner, Callout } from '@blueprintjs/core';
import { makeAutoObservable, runInAction } from 'mobx';
import { fromPromise, IPromiseBasedObservable } from 'mobx-utils';
import { observer } from 'mobx-react-lite';
import {
  Settings,
  FSOSettings,
  FSOSettings_DefaultSettings,
  FSOSettings_VideoSettings,
  HardwareInfoResponse,
  NullMessage,
} from '@api/client';
import { FinishedUnaryCall } from '@protobuf-ts/runtime-rpc';
import { GlobalState, useGlobalState } from '../lib/state';
import FormContext, { useFormContext } from '../elements/form-context';
import { FormCheckbox, FormInputGroup, FormSelect, FormSlider } from '../elements/form-elements';
import ErrorDialog from '../dialogs/error-dialog';

class SettingsState {
  loading = true;
  saving = false;
  error = false;
  errorMessage = '';
  saveError = false;
  gs: GlobalState;
  knSettings: Settings = Settings.create();
  fsoSettings: FSOSettings = FSOSettings.create();

  constructor(gs: GlobalState) {
    makeAutoObservable(this, { gs: false }, { proxy: false });
    this.gs = gs;

    void (async () => {
      try {
        const settings = await gs.client.getSettings({});
        const fsoSettings = await gs.client.loadFSOSettings({});

        runInAction(() => {
          this.knSettings = settings.response;
          this.fsoSettings = fsoSettings.response;
          this.loading = false;
        });
      } catch (e) {
        console.error(e);
        runInAction(() => {
          this.error = true;
          this.loading = false;
          this.errorMessage = e instanceof Error ? e.message : String(e);
        });
      }
    })();
  }

  get resolution(): string {
    const m = /\(([0-9]+)x([0-9]+)\)x([0-9]+) bit/.exec(
      this.fsoSettings.default?.videocardFs2Open ?? '',
    );
    if (!m) {
      return '';
    }

    return `${m[1]}x${m[2]} - ${this.fsoSettings.video?.display ?? '0'}`;
  }

  set resolution(value: string) {
    const m = /([0-9]+)x([0-9]+) - ([0-9]+)/.exec(value);
    if (!m) {
      return;
    }

    const oldValue = /\(([0-9]+)x([0-9]+)\)x([0-9]+) bit/.exec(
      this.fsoSettings.default?.videocardFs2Open ?? '',
    );

    let def = this.fsoSettings.default;
    if (!def) {
      this.fsoSettings.default = FSOSettings_DefaultSettings.create();
      def = this.fsoSettings.default;
    }

    def.videocardFs2Open = `OGL -(${m[1]}x${m[2]})x${oldValue ? oldValue[3] : '32'} bit`;

    let video = this.fsoSettings.video;
    if (!video) {
      this.fsoSettings.video = FSOSettings_VideoSettings.create();
      video = this.fsoSettings.video;
    }

    video.display = parseInt(m[3], 10);
  }

  get depth(): string {
    const m = /\(([0-9]+)x([0-9]+)\)x([0-9]+) bit/.exec(
      this.fsoSettings.default?.videocardFs2Open ?? '',
    );
    if (!m) {
      return '';
    }

    return m[3];
  }

  set depth(value: string) {
    let def = this.fsoSettings.default;
    if (!def) {
      this.fsoSettings.default = FSOSettings_DefaultSettings.create();
      def = this.fsoSettings.default;
    }

    const m = /\(([0-9]+)x([0-9]+)\)x([0-9]+) bit/.exec(def.videocardFs2Open);
    if (m) {
      def.videocardFs2Open = `OGL -(${m[1]}x${m[2]})x${value} bit`;
    }
  }

  get textureFilter(): string {
    return String(this.fsoSettings.default?.textureFilter ?? '');
  }

  set textureFilter(value: string) {
    let def = this.fsoSettings.default;
    if (!def) {
      this.fsoSettings.default = FSOSettings_DefaultSettings.create();
      def = this.fsoSettings.default;
    }

    def.textureFilter = parseInt(value, 10);
  }

  async saveKNSettings() {
    this.saving = true;

    try {
      await this.gs.client.saveSettings(this.knSettings);
      runInAction(() => {
        this.saving = false;
      });
    } catch (e) {
      console.error(e);
      runInAction(() => {
        this.saving = false;

        const message = <pre>{e instanceof Error ? e.message : String(e)}</pre>;
        this.gs.launchOverlay(ErrorDialog, { message });
      });
    }
  }

  async saveFSOSettings() {
    this.saving = true;

    try {
      await this.gs.client.saveFSOSettings(this.fsoSettings);
      runInAction(() => {
        this.saving = false;
      });
    } catch (e) {
      console.error(e);
      runInAction(() => {
        this.saving = false;

        const message = <pre>{e instanceof Error ? e.message : String(e)}</pre>;
        this.gs.launchOverlay(ErrorDialog, { message });
      });
    }
  }

  async saveAll() {
    await this.saveKNSettings();
    await this.saveFSOSettings();
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
  const ctx = useFormContext();
  const value = ctx[props.field] as string;

  return (
    <FormSelect name={props.field} fill={true}>
      {props.hardwareInfo.case({
        pending: () => <option>Loading...</option>,
        rejected: () => <option>ERROR</option>,
        fulfilled: ({ response }) => (
          <>
            <option key="" value="">
              Default
            </option>
            {response[props.infoKey].indexOf(value) < 0 && value !== '' ? (
              <option key={value}>{value}</option>
            ) : null}
            {response[props.infoKey].map((dev) => (
              <option key={dev}>{dev}</option>
            ))}
          </>
        ),
      })}
    </FormSelect>
  );
});

interface VoiceSelectProps {
  hardwareInfo: IPromiseBasedObservable<FinishedUnaryCall<NullMessage, HardwareInfoResponse>>;
}

const VoiceSelect = observer(function VoiceSelect(props: VoiceSelectProps): React.ReactElement {
  return (
    <FormSelect name="speechVoice" fill={true}>
      {props.hardwareInfo.case({
        pending: () => <option>Loading...</option>,
        rejected: () => <option>ERROR</option>,
        fulfilled: ({ response }) => (
          <>
            {response.voices.map((name, i) => (
              <option key={i} value={i}>
                {name}
              </option>
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
    <FormSelect name="currentJoystickGUID" fill={true}>
      {props.hardwareInfo.case({
        pending: () => <option>Loading...</option>,
        rejected: () => <option>ERROR</option>,
        fulfilled: ({ response }) => (
          <>
            <option key="" value="">
              No joystick
            </option>
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

async function selectLibraryFolder(gs: GlobalState, formState: SettingsState): Promise<void> {
  try {
    const result = await knOpenFolder(
      'Please select your library folder',
      formState.knSettings.libraryPath,
    );
    if (result !== '' && result !== formState.knSettings.libraryPath) {
      runInAction(() => {
        formState.knSettings.libraryPath = result;
      });

      const settings = await gs.client.getSettings({});
      settings.response.libraryPath = result;
      await gs.client.saveSettings(settings.response);

      void rescanLocalMods(gs);
    }
  } catch (e) {
    console.error(e);
  }
}

async function rescanLocalMods(gs: GlobalState): Promise<void> {
  try {
    const task = gs.tasks.startTask('Scan new library folder...');
    await gs.client.scanLocalMods({ ref: task });
    gs.sendSignal('showTasks');
  } catch (e) {
    console.error(e);
  }
}

export default observer(function SettingsPage(): React.ReactElement {
  const gs = useGlobalState();
  const [formState] = useState(() => new SettingsState(gs));
  const [hardwareInfo] = useState(() => fromPromise(gs.client.getHardwareInfo({})));

  return (
    <div className="text-white text-sm">
      <div className="flex flex-row gap-4">
        {formState.loading ? (
          <div className="absolute top-0 left-0 right-0 bottom-0 flex justify-center align-middle">
            <Spinner />
          </div>
        ) : formState.error ? (
          <Callout intent="danger" title="Error">
            Failed to load settings.
            <pre>{formState.errorMessage}</pre>
          </Callout>
        ) : (
          <>
            <div className="flex flex-1 flex-col gap-4">
              <Card className="relative">
                <Button className="absolute top-4 right-4" onClick={() => void formState.saveAll()}>
                  Save All
                </Button>

                <h5 className="text-xl mb-5">General Knossos Settings</h5>
                <FormContext value={formState.knSettings as unknown as Record<string, unknown>}>
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
                </FormContext>
              </Card>
              <Card>
                <h5 className="text-xl mb-5">Downloads</h5>
                <FormContext value={formState.knSettings as unknown as Record<string, unknown>}>
                  <div className="flex flex-row gap-4">
                    <FormGroup className="flex-1" label="Max Downloads">
                      <FormInputGroup type="number" name="maxDownloads" />
                    </FormGroup>

                    <FormGroup className="flex-1" label="Download bandwidth limit">
                      <FormInputGroup
                        type="number"
                        name="bandwidthLimit"
                        rightElement={<Tag minimal={true}>KiB/s</Tag>}
                      />
                    </FormGroup>
                  </div>
                </FormContext>
              </Card>

              <Card>
                <h5 className="text-xl mb-5">Video</h5>

                <FormContext
                  value={formState.fsoSettings.default as unknown as Record<string, unknown>}
                >
                  <div className="flex flex-row gap-4">
                    <FormContext value={formState as unknown as Record<string, unknown>}>
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
                    </FormContext>

                    <FormGroup label="Texture Filter">
                      <FormSelect name="textureFilter">
                        <option value="3">Trilinear</option>
                        <option value="2">Bilinear</option>
                      </FormSelect>
                    </FormGroup>
                  </div>
                </FormContext>
              </Card>
            </div>
            <div className="flex flex-1 flex-col gap-4">
              <Card>
                <h5 className="text-xl mb-5">Audio</h5>
                <FormContext
                  value={formState.fsoSettings.sound as unknown as Record<string, unknown>}
                >
                  <FormGroup label="Playback Device">
                    <HardwareSelect
                      hardwareInfo={hardwareInfo}
                      infoKey="audioDevices"
                      field="playbackDevice"
                    />
                  </FormGroup>

                  <FormGroup label="Capture Device">
                    <HardwareSelect
                      hardwareInfo={hardwareInfo}
                      infoKey="captureDevices"
                      field="captureDevice"
                    />
                  </FormGroup>

                  <FormCheckbox name="enableEFX" label="Enable EFX" />

                  <div className="flex flex-row gap-4">
                    <FormGroup className="flex-1" label="Sample Rate">
                      <FormInputGroup type="number" name="sampleRate" />
                    </FormGroup>

                    <FormContext
                      value={formState.fsoSettings.default as unknown as Record<string, unknown>}
                    >
                      <FormGroup className="flex-1" label="Language">
                        <FormSelect name="language">
                          <option>English</option>
                        </FormSelect>
                      </FormGroup>
                    </FormContext>
                  </div>
                </FormContext>
              </Card>

              <Card>
                <h5 className="text-xl mb-5">Speech</h5>
                <FormContext
                  value={formState.fsoSettings.default as unknown as Record<string, unknown>}
                >
                  <div className="flex flex-row gap-4">
                    <div className="flex-1">
                      <FormGroup label="Voice">
                        <VoiceSelect hardwareInfo={hardwareInfo} />
                      </FormGroup>

                      <FormGroup label="Volume">
                        <FormSlider min={0} max={100} labelStepSize={10} name="speechVolume" />
                      </FormGroup>
                    </div>

                    <FormGroup className="flex-1" label="Use Speech in">
                      <FormCheckbox name="speechTechroom" label="Tech Room" />
                      <FormCheckbox name="speechIngame" label="In-Game" />
                      <FormCheckbox name="speechBriefings" label="Briefings" />
                      <FormCheckbox name="speechMulti" label="Multiplayer" />
                    </FormGroup>
                  </div>
                </FormContext>
              </Card>

              <Card>
                <h5 className="text-xl mb-5">Joystick</h5>

                <FormContext
                  value={formState.fsoSettings.default as unknown as Record<string, unknown>}
                >
                  <FormGroup label="Joystick">
                    <JoystickSelect hardwareInfo={hardwareInfo} />
                  </FormGroup>

                  <FormCheckbox name="enableJoystickFF" label="Force Feedback" />
                  <FormCheckbox name="enableHitEffect" label="Directional Hit" />
                </FormContext>
              </Card>
            </div>
          </>
        )}
      </div>
    </div>
  );
});
