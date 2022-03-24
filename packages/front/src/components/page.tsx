import React from 'react';
import {
  H1,
  Navbar,
  NavbarGroup,
  NavbarHeading,
  NavbarDivider,
  Menu,
  MenuItem,
  Button,
} from '@blueprintjs/core';
import { Popover2 } from '@blueprintjs/popover2';
import { Routes, Route, useNavigate } from 'react-router-dom';
import { Observer } from 'mobx-react-lite';

import { useGlobalState } from '../lib/state';
import { AlertContainer } from '../lib/alert';
import Register from '../pages/register';
import Login from '../pages/login';
import ResetPassword from '../pages/reset-password';
import ResetPasswordContinued from '../pages/reset-password-continue';
import Logout from '../pages/logout';
import ModList from '../pages/mods/list';
import ModDetails from '../pages/mods/details';

interface Props {
  children?: React.ReactNode | React.ReactNode[];
}

export default function Page(_props: Props): React.ReactElement {
  const gs = useGlobalState();
  const navigate = useNavigate();

  return (
    <>
      <Navbar>
        <div className="mx-auto max-w-screen-lg w-full">
          <NavbarGroup>
            <NavbarHeading>Neo Nebula</NavbarHeading>
            <NavbarDivider />
            <Button minimal icon="home" text="Home" onClick={() => navigate('/')} />
            <Button minimal icon="widget" text="Mods" onClick={() => navigate('/mods')} />
            <Observer>
              {() =>
                gs.user?.loggedIn ? (
                  <Popover2
                    placement="bottom"
                    minimal
                    content={
                      <Menu>
                        <MenuItem icon="plus" text="Create Mod" />
                        <MenuItem icon="people" text="My Mods" />
                      </Menu>
                    }
                  >
                    <Button minimal icon="chevron-down" />
                  </Popover2>
                ) : null
              }
            </Observer>
          </NavbarGroup>
          <NavbarGroup align="right">
            <Observer>
              {() =>
                gs.user?.loggedIn ? (
                  <>
                    {gs.user.username}
                    <Button minimal={true} icon="log-out" onClick={() => navigate('/logout')}>
                      Logout
                    </Button>
                  </>
                ) : (
                  <>
                    <Button minimal={true} icon="log-in" onClick={() => navigate('/login')}>
                      Login
                    </Button>
                    <Button minimal={true} icon="key" onClick={() => navigate('/register')}>
                      Register
                    </Button>
                  </>
                )
              }
            </Observer>
          </NavbarGroup>
        </div>
      </Navbar>

      <div className="container py-5 max-w-screen-lg mx-auto">
        <Routes>
          <Route
            path="/"
            element={
              <>
                <H1>Welcome back!</H1>
                <p>Well, here we go again...</p>
                <p>Let's hope this attempt works out better than last time.</p>
              </>
            }
          />
          <Route path="/register" element={<Register />} />
          <Route path="/login" element={<Login />} />
          <Route path="/login/reset-password" element={<ResetPassword />} />
          <Route path="/mail/reset/:token" element={<ResetPasswordContinued />} />
          <Route path="/logout" element={<Logout />} />
          <Route path="/mods" element={<ModList />} />
          <Route path="/mod/:modid/:version" element={<ModDetails />} />
          <Route path="/mod/:modid" element={<ModDetails />} />
          <Route path="*">
            <H1>Not Found</H1>
            <p>I'm sorry but I could not find what you're looking for.</p>
          </Route>
        </Routes>
      </div>

      <AlertContainer />
    </>
  );
}
