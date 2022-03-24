import React, { useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { H1, Spinner } from '@blueprintjs/core';

import { useGlobalState } from '../lib/state';

export default function LoginPage(): React.ReactElement {
  const gs = useGlobalState();
  const navigate = useNavigate();

  useEffect(() => {
    gs.user?.logout();
    navigate('/');

    gs.toaster.show({
      message: "You're now logged out.",
      intent: 'success',
    });
  }, [gs.user, navigate, gs.toaster]);

  return (
    <div className="max-w-md">
      <H1>We're logging you out...</H1>
      <Spinner intent="primary" />
    </div>
  );
}
