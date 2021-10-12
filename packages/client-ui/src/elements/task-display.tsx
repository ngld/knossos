import { useState, useEffect, useRef } from 'react';
import { Classes, Button, Dialog, ProgressBar, Text } from '@blueprintjs/core';
import { CiTimesLine } from '@meronex/icons/ci';
import { Tooltip2 } from '@blueprintjs/popover2';
import { action } from 'mobx';
import { observer } from 'mobx-react-lite';
import { useGlobalState, GlobalState } from '../lib/state';
import { TaskState } from '../lib/task-tracker';

interface LogBoxProps {
  task: TaskState;
}
function LogBox({ task }: LogBoxProps): React.ReactElement {
  const boxRef = useRef<HTMLDivElement>(null);
  const isBottom = useRef<boolean>(true);

  useEffect(() => {
    const box = boxRef.current;
    if (box) {
      box.appendChild(task.logContainer);

      const obs = new MutationObserver(() => {
        if (box && isBottom.current) {
          box.scrollTop = box.scrollHeight;
        }
      });
      obs.observe(box, {
        childList: true,
        attributes: false,
        characterData: false,
        subtree: true,
      });

      return () => obs.disconnect();
    }

    return () => void 0;
  }, [boxRef, task.logContainer]);

  const scrollHandler = () => {
    const box = boxRef.current;
    if (box) {
      // 10 pixel tolerance
      isBottom.current = box.scrollTop + box.clientHeight + 10 >= box.scrollHeight;
    }
  };

  // Make sure TailwindCSS includes this class
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  const _ = 'font-mono';
  return (
    <div
      ref={boxRef}
      className="overflow-y-auto max-h-56 bg-base text-xs"
      onScroll={scrollHandler}
    />
  );
}

const clearTasks = action(function clearTasks(gs: GlobalState): void {
  const taskIDs = gs.tasks.tasks.map((task) => task.id);
  for (const id of taskIDs) {
    gs.tasks.removeTask(id);
  }
});

export default observer(function TaskDisplay(): React.ReactElement {
  const gs = useGlobalState();
  const [open, setOpen] = useState<boolean>(false);

  gs.useSignal('showTasks', () => setOpen(true));
  gs.useSignal('hideTasks', () => setOpen(false));

  return (
    <div className="absolute bottom-0 right-40 no-window-drag">
      <Tooltip2 content="Show active tasks">
        <div
          className={'cursor-pointer ' + (gs.tasks.active > 0 ? 'text-white' : 'text-dim')}
          onClick={() => setOpen(!open)}
        >
          {gs.tasks.active === 0
            ? 'No active tasks'
            : gs.tasks.active > 1
            ? `${gs.tasks.active} background tasks`
            : gs.tasks.tasks[0].label}
        </div>
      </Tooltip2>
      <Dialog
        isOpen={open}
        onClose={() => setOpen(false)}
        title={
          <>
            Tasks{' '}
            <Button className="float-right" minimal={true} onClick={() => clearTasks(gs)}>
              Clear All
            </Button>
          </>
        }
        className="large-dialog"
      >
        <div className={Classes.DIALOG_BODY}>
          {gs.tasks.tasks.map((task) => (
            <div key={task.id}>
              <div className="relative">
                <Text className="mb-1" ellipsize={true}>
                  {task.label} {task.status !== '' ? ' - ' + task.status : ''}
                </Text>
                {(task.progress === 1 || task.error) && (
                  <div className="absolute right-0 top-0">
                    <Button minimal small onClick={() => gs.tasks.cancelTask(task.id)}>
                      Cancel
                    </Button>
                    <Button
                      minimal
                      small
                      disabled={(task.indeterminate || task.progress < 1) && !task.error}
                      onClick={() => gs.tasks.removeTask(task.id)}
                    >
                      <CiTimesLine />
                    </Button>
                  </div>
                )}
              </div>
              <ProgressBar
                stripes={task.indeterminate}
                value={task.indeterminate ? 1 : task.progress}
                intent={task.error ? 'danger' : task.progress === 1 ? 'success' : 'primary'}
              />
              <LogBox task={task} />
            </div>
          ))}
        </div>
      </Dialog>
    </div>
  );
});
