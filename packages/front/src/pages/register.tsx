import React from 'react';
import { Callout, Divider } from '@blueprintjs/core';
import { useNavigate, NavigateFunction } from 'react-router-dom';
import { RpcError } from '@protobuf-ts/runtime-rpc';

import type { BoolResponse } from '@api/service';
import { useGlobalState, GlobalState } from '../lib/state';
import { alert } from '../lib/alert';
import { presentTwirpError } from '../lib/twirp-helpers';
import { Form, Field, FormButton, Errors, DefaultOptions } from '../components/form';

interface FormState {
  email: string;
  user: string;
  password: string;
  passwordRepeat: string;
}

function validate(state: FormState): Errors<FormState> {
  const errors: Errors<FormState> = {};
  for (const [key, value] of Object.entries(state)) {
    if (value === '') {
      errors[key as keyof FormState] = 'This field is required.';
    }
  }

  if (!errors.passwordRepeat) {
    if (state.password !== state.passwordRepeat) {
      errors.passwordRepeat = "The entered password doesn't match the previous field.";
    }
  }

  return errors;
}

async function submitForm(
  state: FormState,
  defaults: DefaultOptions,
  navigate: NavigateFunction,
  gs: GlobalState,
) {
  defaults.spinning = true;
  defaults.disabled = true;
  let response: BoolResponse | undefined;
  try {
    response = (
      await gs.client.register({
        email: state.email,
        username: state.user,
        password: state.password,
      })
    ).response;
  } catch (e) {
    console.log(e);

    if (e instanceof RpcError) {
      defaults.disabled = false;
      defaults.spinning = false;

      presentTwirpError(e.code);
      return;
    }
  }

  console.log(response);

  if (!response?.success) {
    alert({
      children: 'Failed to register. Most likely this username is already taken.',
      confirmButtonText: 'OK',
    });

    defaults.disabled = false;
    defaults.spinning = false;
  } else {
    gs.toaster.show(
      {
        message: 'User account created. Please check your inbox for the verification mail.',
        intent: 'success',
      },
      'register-success',
    );

    defaults.spinning = false;
    navigate('/');
  }
}

export default function RegisterPage(): React.ReactElement {
  const gs = useGlobalState();
  const navigate = useNavigate();

  return (
    <div className="flex container gap-4">
      <div className="flex-1">
        <Form
          initialState={
            {
              user: '',
              email: '',
              password: '',
              passwordRepeat: '',
            } as FormState
          }
          onValidate={validate}
          onSubmit={(s, d) => void submitForm(s, d, navigate, gs)}
        >
          <Field name="user" label="Username" required />
          <Field name="email" label="E-Mail" type="email" required />
          <Field name="password" label="Password" type="password" required />
          <Field
            name="passwordRepeat"
            label="Password repeat"
            type="password"
            helperText="Just repeat the previous password to make sure that you entered it correctly."
            required
          />

          <FormButton type="submit" intent="primary">
            Register
          </FormButton>
        </Form>
      </div>
      <Divider className="flex-initial" />
      <div className="flex-1">
        <Callout title="Important note regarding the E-Mail field">
          You have to specify a valid email address since we'll mail you a validation link to check
          that this address actually belongs to you.
        </Callout>
      </div>
    </div>
  );
}
