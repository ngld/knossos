import { useState } from 'react';
import { Dialog, Callout, Button, Classes } from '@blueprintjs/core';
import { TaskResult } from '@api/client';
import { UnaryCall } from '@protobuf-ts/runtime-rpc';
import { GlobalState } from '../lib/state';

interface ErrorDialogProps {
  title?: string;
  message: string | React.ReactNode;
  onFinished?: () => void;
}
export default function ErrorDialog(props: ErrorDialogProps): React.ReactElement {
  const [isOpen, setOpen] = useState(true);

  return (
    <Dialog
      className="bp3-ui-text large-dialog"
      isOpen={isOpen}
      onClose={() => setOpen(false)}
      onClosed={() => {
        if (props.onFinished) {
          props.onFinished();
        }
      }}
    >
      <div className={Classes.DIALOG_BODY}>
        <Callout className="overflow-auto" intent="danger" title={props.title ?? 'Error'}>
          {props.message}
        </Callout>
      </div>
      <div className={Classes.DIALOG_FOOTER}>
        <div className={Classes.DIALOG_FOOTER_ACTIONS}>
          <Button intent="primary" onClick={() => setOpen(false)}>
            Close
          </Button>
        </div>
      </div>
    </Dialog>
  );
}

export function maybeError(gs: GlobalState, result: TaskResult | Promise<TaskResult> | UnaryCall<any, TaskResult>): void {
  if ('request' in result) {
    void result.then((r) => maybeError(gs, r.response));
    return;
  }

  if ('then' in result) {
    void result.then((r) => maybeError(gs, r));
    return;
  }

  if (!result.success) {
    gs.launchOverlay(ErrorDialog, { message: result.error });
  }
}
