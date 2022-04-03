import { createContext, useEffect } from 'react';
import { makeAutoObservable } from 'mobx';
import { TwirpFetchTransport } from '@protobuf-ts/twirp-transport';
import { Toaster, IToaster } from '@blueprintjs/core';
import { KnossosClient } from '@api/client.client';
import { NebulaClient } from '@api/service.client';
import { TaskTracker } from './task-tracker';
import { API_URL } from './constants';

interface OverlayProps {
  onFinished?: () => void;
}

type SignalName =
  | 'remoteRefreshMods'
  | 'hideTasks'
  | 'showTasks'
  | 'reloadLocalMods'
  | 'reloadRemoteMods';

export class GlobalState {
  toaster: IToaster;
  client: KnossosClient;
  nebula: NebulaClient;
  tasks: TaskTracker;
  _nextOverlayID = 0;
  overlays: [
    React.FunctionComponent<OverlayProps> | React.ComponentClass<OverlayProps>,
    Record<string, unknown>,
    number,
  ][];
  signalListeners: Record<SignalName, (() => void)[]> = {
    remoteRefreshMods: [],
    showTasks: [],
    hideTasks: [],
    reloadLocalMods: [],
    reloadRemoteMods: [],
  };
  startupDone: boolean;

  constructor() {
    this.toaster = Toaster.create({});
    this.client = new KnossosClient(
      new TwirpFetchTransport({
        baseUrl: API_URL + '/twirp',
        deadline: 1000,
      }),
    );
    this.nebula = new NebulaClient(
      new TwirpFetchTransport({
        baseUrl:
          process.env.NODE_ENV === 'production'
            ? 'https://nu.fsnebula.org/twirp'
            : 'http://localhost:8200/twirp',
        deadline: process.env.NODE_ENV === 'production' ? 10000 : 1000,
      }),
    );
    this.tasks = new TaskTracker(this);
    this.tasks.listen();
    this.overlays = [];
    this.startupDone = false;

    makeAutoObservable(this, {
      // don't use black magic on complex objects
      toaster: false,
      client: false,
      nebula: false,
      tasks: false,
      // don't let MobX mess with the stored callbacks
      signalListeners: false,
    });
  }

  launchOverlay<T extends OverlayProps = OverlayProps>(
    component:
      | React.FunctionComponent<T>
      | React.ComponentClass<T>
      | ((props: T) => React.ReactNode),
    props: T,
  ): number {
    const idx = this.overlays.length;
    this.overlays.push([
      component as React.FunctionComponent<OverlayProps>,
      props as Record<string, unknown>,
      this._nextOverlayID++,
    ]);
    return idx;
  }

  removeOverlay(index: number): void {
    this.overlays.splice(index, 1);
  }

  useSignal(name: SignalName, listener: () => void): void {
    // This is not a class component so this error is bogus.
    // eslint-disable-next-line react-hooks/rules-of-hooks
    useEffect(() => {
      this.signalListeners[name].push(listener);
      return () => {
        const pos = this.signalListeners[name].indexOf(listener);
        if (pos === -1) {
          console.error(
            this.signalListeners[name].map((cb) => cb.toString()),
            listener.toString(),
            `Signal listener for ${name} vanished?!`,
          );
          return;
        }

        this.signalListeners[name].splice(pos, 1);
      };
    });
  }

  sendSignal(name: SignalName): void {
    for (const listener of this.signalListeners[name]) {
      listener();
    }
  }
}

const globalStateCtx = createContext<GlobalState | null>(null);
globalStateCtx.displayName = 'StateContext';

export const StateProvider = globalStateCtx.Provider;
export function useGlobalState(): GlobalState {
  return gs;
}

export const gs = new GlobalState();
