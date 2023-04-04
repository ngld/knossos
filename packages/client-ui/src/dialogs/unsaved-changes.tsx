import { useState } from 'react';
import { Dialog, Callout, Button, Classes } from '@blueprintjs/core';
import cx from 'classnames';

interface UnsaveChangesProps {
  title?: string;
  message: string | React.ReactNode;
  onResolve: (save: boolean, cancel: boolean) => void;
  onFinished?: () => void;
}
export default function UnsaveChangesDialog(props: UnsaveChangesProps): React.ReactElement {
  const [isOpen, setOpen] = useState(true);

  return (
    <Dialog
      className={cx(Classes.UI_TEXT)}
      isOpen={isOpen}
      onClose={() => setOpen(false)}
      onClosed={() => {
        if (props.onFinished) {
          props.onFinished();
        }
      }}
    >
      <div className={Classes.DIALOG_BODY}>
        You have unsaved changes. Do you want to save them now or discard them?
      </div>
      <div className={Classes.DIALOG_FOOTER}>
        <div className={Classes.DIALOG_FOOTER_ACTIONS}>
          <Button
            intent="primary"
            onClick={() => {
              setOpen(false);
              props.onResolve(true, false);
            }}
          >
            Save
          </Button>
          <Button
            intent="danger"
            onClick={() => {
              setOpen(false);
              props.onResolve(false, false);
            }}
          >
            Discard
          </Button>
          <Button
            onClick={() => {
              setOpen(false);
              props.onResolve(false, true);
            }}
          >
            Cancel
          </Button>
        </div>
      </div>
    </Dialog>
  );
}
