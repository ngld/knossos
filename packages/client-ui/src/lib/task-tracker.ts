import { useState, useEffect } from 'react';
import { makeObservable, action, observable, computed } from 'mobx';
import EventEmitter from 'eventemitter3';
import { LogMessage, LogMessage_LogLevel, ClientSentEvent } from '@api/client';
import { GlobalState } from '../lib/state';

export interface TaskState {
  id: number;
  label: string;
  progress: number;
  status: string;
  error: boolean;
  indeterminate: boolean;
  started: number;
  canCancel: boolean;
  logMessages: LogMessage[];
  logContainer: HTMLDivElement,
  finishCb?: (success: boolean) => void;
}

export const logLevelMap: Record<LogMessage_LogLevel, string> = {} as Record<
  LogMessage_LogLevel,
  string
>;
for (const [name, level] of Object.entries(LogMessage_LogLevel)) {
  logLevelMap[level as LogMessage_LogLevel] = name;
}

function getLogTime(task: TaskState, line: LogMessage): string {
  const time = line.time;
  if (!time) {
    return '00:00';
  }

  const duration = time.seconds - task.started;
  const minutes = Math.floor(duration / 60);
  const seconds = duration % 60;

  let result = (minutes < 10 ? '0' : '') + String(minutes) + ':';
  result += (seconds < 10 ? '0' : '') + String(seconds);
  return result;
}

export class TaskTracker extends EventEmitter {
  _idCounter: number;
  _gs: GlobalState;
  tasks: TaskState[];
  taskMap: Record<string, TaskState>;

  constructor(gs: GlobalState) {
    super();
    this._gs = gs;
    this._idCounter = 1;
    this.tasks = [];
    this.taskMap = {};

    makeObservable(this, {
      _gs: false,
      tasks: observable,
      taskMap: observable,
      active: computed,
      listen: action,
      startTask: action,
      updateTask: action,
      removeTask: action,
    });
  }

  get active(): number {
    let count = 0;
    for (const task of this.tasks) {
      if (task.progress < 1 && !task.error) {
        count++;
      }
    }
    return count;
  }

  listen(): () => void {
    const listener = action((queue: ArrayBuffer[]) => {
      if (!Array.isArray(queue)) {
        console.error('Invalid queue passed to listener()!');
      }
      try {
        for (const msg of queue) {
          const ev = ClientSentEvent.fromBinary(new Uint8Array(msg));
          this.updateTask(ev);
        }
      } catch (e) {
        console.error(e);
      }
    });

    knAddMessageListener(listener);
    return () => knRemoveMessageListener(listener);
  }

  startTask(label: string, finishCb?: (success: boolean) => void, canCancel = false): number {
    const id = this._idCounter++;
    const task = {
      id,
      label,
      progress: 0,
      status: 'Initialising',
      error: false,
      indeterminate: true,
      started: Math.floor(Date.now() / 1000),
      canCancel,
      logMessages: [],
      logContainer: document.createElement('div'),
      finishCb,
    } as TaskState;

    this.taskMap[id] = task;
    this.tasks.unshift(this.taskMap[id]);
    this.emit('new', id);

    return id;
  }

  runTask(label: string, launcher: (id: number) => void): Promise<boolean> {
    return new Promise((resolve) => {
      const id = this.startTask(label, resolve);
      launcher(id);
    });
  }

  updateTask(ev: ClientSentEvent): void {
    const task = this.taskMap[ev.ref];
    if (!task) {
      console.error(`Got update for missing task ${ev.ref}`);
      return;
    }

    let msg: LogMessage;
    let line: HTMLDivElement;
    let lineText: HTMLSpanElement;

    switch (ev.payload.oneofKind) {
      case 'message':
        // task.logMessages.push(ev.payload.message);
        msg = ev.payload.message;
        line = document.createElement('div');
        line.setAttribute('title', msg.sender);
        line.setAttribute('class', 'log-' + (logLevelMap[msg.level] ?? 'info').toLowerCase());

        lineText = document.createElement('span');
        lineText.setAttribute('class', 'font-mono');
        lineText.innerText = `[${getLogTime(task, msg)}]:`;

        line.appendChild(lineText);
        line.innerHTML += '&nbsp;' + msg.message
          .replace(/</g, '&lt;')
          .replace(/>/g, '&gt;')
          .replace(/"/g, '&quot;')
          .replace(/\n/g, '<br>')
          .replace(/\t/g, '    ')
          .replace(/ [ ]+/g, (m) => {
            let result = '';
            for (let i = 0; i < m.length; i++) {
              result += '&nbsp;';
            }
            return result;
          });

        task.logContainer.appendChild(line);
        break;
      case 'progress':
        {
          const info = ev.payload.progress;
          if (info.progress >= 0) {
            task.progress = info.progress;
          }
          if (info.description !== '') {
            task.status = info.description;
          }
          task.error = info.error;
          task.indeterminate = info.indeterminate;
        }
        break;
      case 'result':
        {
          const taskResult = ev.payload.result;
          task.indeterminate = false;

          if (!taskResult.success) {
            task.error = true;
            task.status = 'Failed';
          } else {
            task.progress = 1;
            task.status = 'Done';
          }
          if (task.finishCb) {
            task.finishCb(taskResult.success);
          }
        }
        break;
    }

    this.taskMap[ev.ref] = task;
  }

  cancelTask(id: number): void {
    void this._gs.client.cancelTask({ ref: id });
  }

  removeTask(id: number): void {
    let taskIdx = -1;
    for (let i = 0; i < this.tasks.length; i++) {
      if (this.tasks[i].id === id) {
        taskIdx = i;
        break;
      }
    }

    if (taskIdx === -1) {
      console.error(`Task with id ${id} not found in the current task list.`);
      return;
    }

    this.tasks.splice(taskIdx, 1);
    delete this.taskMap[id];
  }
}

export function useTaskTracker(gs: GlobalState): TaskTracker {
  const [tracker] = useState(() => new TaskTracker(gs));

  useEffect(() => {
    return tracker.listen();
  }, [tracker]);

  return tracker;
}
