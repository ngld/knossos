import { useState } from 'react';
import { Dialog, Callout, Button, Classes } from '@blueprintjs/core';

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
