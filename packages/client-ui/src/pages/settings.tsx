import { useState, useEffect } from 'react';
import { Button, Card, ControlGroup, FormGroup } from '@blueprintjs/core';
import { makeAutoObservable, autorun, runInAction } from 'mobx';
import { Settings, TaskRequest } from '@api/client';
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

  useEffect(() => {
    void loadSettings(gs, formState);

    return autorun(() => {
      console.log('Settings changed...');
      void saveSettings(gs, formState);
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
              <FormGroup label="Max Downloads">
                <FormInputGroup name="maxDownloads" />
              </FormGroup>

              <FormGroup label="Download bandwidth limit">
                <ControlGroup fill={true}>
                  <FormInputGroup name="bandwidthLimit" />
                  <span>KiB/s</span>
                </ControlGroup>
              </FormGroup>
            </Card>

            <Card>
              <h5 className="text-xl mb-5">Video</h5>

              <div className="flex flex-row gap-4">
                <FormGroup label="Resolution">
                  <FormSelect name="resolution">
                    <option>TODO</option>
                  </FormSelect>
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
                <FormSelect name="playbackDevice" fill={true}>
                  <option>TODO</option>
                </FormSelect>
              </FormGroup>

              <FormGroup label="Capture Device">
                <FormSelect name="captureDevice" fill={true}>
                  <option>TODO</option>
                </FormSelect>
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
                    <FormSelect name="voice" fill={true}>
                      <option>TODO</option>
                    </FormSelect>
                  </FormGroup>

                  <FormGroup label="Volume">TODO</FormGroup>
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
                <FormSelect name="joystick" fill={true}>
                  <option>TODO</option>
                </FormSelect>
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
