import { createContext, useContext } from 'react';
import {makeAutoObservable} from 'mobx';
import { TwirpFetchTransport } from '@protobuf-ts/twirp-transport';
import { Toaster, IToaster } from '@blueprintjs/core';
import { KnossosClient } from '@api/client.client';
import { NebulaClient } from '@api/service.client';
import { TaskTracker } from './task-tracker';
import { API_URL } from './constants';

interface OverlayProps {
  onFinished?: () => void;
}

export class GlobalState {
  toaster: IToaster;
  client: KnossosClient;
  nebula: NebulaClient;
  tasks: TaskTracker;
  overlays: [React.FunctionComponent<OverlayProps> | React.ComponentClass<OverlayProps>, Record<string, unknown>][];

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
        baseUrl: process.env.NODE_ENV === 'production' ? 'https://nu.fsnebula.org/twirp' : 'http://localhost:8200/twirp',
        deadline: process.env.NODE_ENV === 'production' ? 10000 : 1000,
      }),
    );
    this.tasks = new TaskTracker();
    this.tasks.listen();
    this.overlays = [];

    makeAutoObservable(this);
  }

  launchOverlay<T extends OverlayProps = OverlayProps>(component: React.FunctionComponent<T> | React.ComponentClass<T> | ((props: T) => React.ReactNode), props: T): void {
    this.overlays.push([component as React.FunctionComponent<OverlayProps>, props as Record<string, unknown>]);
  }

  removeOverlay(index: number): void {
    this.overlays.splice(index, 1);
  }
}

const globalStateCtx = createContext<GlobalState | null>(null);
globalStateCtx.displayName = 'StateContext';

export const StateProvider = globalStateCtx.Provider;
export function useGlobalState(): GlobalState {
  const ctx = useContext(globalStateCtx);
  if (ctx === null) {
    throw new Error('StateContext is missing!');
  }

  return ctx;
}
