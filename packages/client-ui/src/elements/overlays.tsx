import { useState } from 'react';
import { Dialog, Alert, DialogProps, AlertProps } from '@blueprintjs/core';

interface OverlayDialogProps extends Omit<DialogProps, 'isOpen'> {
  onFinished?: () => void;
  children: React.ReactNode | React.ReactNode[];
}

export function OverlayDialog(props: OverlayDialogProps): React.ReactElement {
  const [isOpen, setOpen] = useState(true);
  return (
    <Dialog
      className="bp3-ui-text"
      isOpen={isOpen}
      onClose={(e) => {
        setOpen(false);
        if (props.onClose) {
          props.onClose(e);
        }
      }}
      onClosed={(e) => {
        if (props.onFinished) {
          props.onFinished();
        }
        if (props.onClosed) {
          props.onClosed(e);
        }
      }}
    >
      {props.children}
    </Dialog>
  );
}

interface OverlayAlertProps extends Omit<AlertProps, 'isOpen'> {
  onFinished?: () => void;
  children: React.ReactNode | React.ReactNode[];
}

export function OverlayAlert(props: OverlayAlertProps): React.ReactElement {
  const [isOpen, setOpen] = useState(true);
  return (
    <Alert
      className="large-dialog"
      isOpen={isOpen}
      onClose={(e) => {
        setOpen(false);
        if (props.onClose) {
          props.onClose(e);
        }
      }}
      onClosed={(e) => {
        if (props.onFinished) {
          props.onFinished();
        }
        if (props.onClosed) {
          props.onClosed(e);
        }
      }}
    >
      {props.children}
    </Alert>
  );
}
