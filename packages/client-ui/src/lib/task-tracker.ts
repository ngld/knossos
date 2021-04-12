import { useState, useEffect } from 'react';
import { makeObservable, action, observable, computed } from 'mobx';
import EventEmitter from 'eventemitter3';
import { LogMessage, LogMessage_LogLevel, ClientSentEvent } from '@api/client';

export interface TaskState {
  id: number;
  label: string;
  progress: number;
  status: string;
  error: boolean;
  indeterminate: boolean;
  started: number;
  logMessages: LogMessage[];
}

export const logLevelMap: Record<LogMessage_LogLevel, string> = {} as Record<
  LogMessage_LogLevel,
  string
>;
for (const [name, level] of Object.entries(LogMessage_LogLevel)) {
  logLevelMap[level as LogMessage_LogLevel] = name;
}

export class TaskTracker extends EventEmitter {
  _idCounter: number;
  tasks: TaskState[];
  taskMap: Record<string, TaskState>;

  constructor() {
    super();
    this._idCounter = 1;
    this.tasks = [];
    this.taskMap = {};

    makeObservable(this, {
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
    const listener = (msg: ArrayBuffer) => {
      try {
        const ev = ClientSentEvent.fromBinary(new Uint8Array(msg));
        this.updateTask(ev);
      } catch (e) {
        console.error(e);
      }
    };

    knAddMessageListener(listener);
    return () => knRemoveMessageListener(listener);
  }

  startTask(label: string): number {
    const id = this._idCounter++;
    const task = {
      id,
      label,
      progress: 0,
      status: 'Initialising',
      error: false,
      indeterminate: true,
      started: Math.floor(Date.now() / 1000),
      logMessages: [],
    } as TaskState;

    this.taskMap[id] = task;
    this.tasks.unshift(this.taskMap[id]);
    this.emit('new', id);

    return id;
  }

  updateTask(ev: ClientSentEvent): void {
    const task = this.taskMap[ev.ref];
    if (!task) {
      console.error(`Got update for missing task ${ev.ref}`);
      return;
    }

    switch (ev.payload.oneofKind) {
      case 'message':
        task.logMessages.push(ev.payload.message);
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
          } else {
            task.progress = 1;
          }
        }
        break;
    }

    this.taskMap[ev.ref] = task;
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

export function useTaskTracker(): TaskTracker {
  const [tracker] = useState(() => new TaskTracker());

  useEffect(() => {
    return tracker.listen();
  }, [tracker]);

  return tracker;
}
