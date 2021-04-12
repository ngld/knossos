import React, { useMemo } from 'react';
import { Callout, Spinner } from '@blueprintjs/core';
import type { RouteComponentProps } from 'react-router-dom';
import type { History } from 'history';
import { fromPromise } from 'mobx-utils';
import { observer } from 'mobx-react-lite';

import { useGlobalState, GlobalState } from '../lib/state';
import { alert } from '../lib/alert';

async function sendValidation(gs: GlobalState, history: History, token: string): Promise<boolean> {
  const response = await gs.runTwirpRequest(gs.client.verifyAccount.bind(gs.client), {
    token,
  });

  if (response?.success) {
    gs.toaster.show({
      message: 'Successfully verified mail. You can now login.',
      icon: 'confirm',
      intent: 'success',
    });
    return true;
  } else {
    alert({
      icon: 'error',
      intent: 'danger',
      children: ['Failed to verify'],
    });
    return false;
  }
}

interface Params {
  token: string;
}

export default observer(function VerifyMailPage(
  props: RouteComponentProps<Params>,
): React.ReactElement {
  const gs = useGlobalState();
  const validation = useMemo(
    () => fromPromise(sendValidation(gs, props.history, props.match.params.token)),
    [gs, props.history, props.match.params.token],
  );

  return (
    <div className="container">
      {validation.case({
        pending: () => <Spinner />,
        rejected: () => (
          <Callout intent="danger" title="Error">
            Failed to contact the server. Please reload the page to try again.
          </Callout>
        ),
        fulfilled: (result: boolean) => (
          <Callout intent={result ? 'success' : 'danger'} title="Done">
            {result
              ? 'Sucessfully verified. You can login now.'
              : "Failed to verify. Make sure you haven't used this link before. Try logging in, just in case."}
          </Callout>
        ),
      })}
    </div>
  );
});
