import React from 'react';
import { Callout } from '@blueprintjs/core';

interface ErrorWrapperProps {
  children: React.ReactNode[] | React.ReactNode;
}
interface ErrorWrapperState {
  error: Error | null;
  info: React.ErrorInfo | null;
}

export default class ErrorBoundary extends React.Component<ErrorWrapperProps, ErrorWrapperState> {
  constructor(props: ErrorWrapperProps) {
    super(props);
    this.state = { error: null, info: null };
  }

  static getDerivedStateFromError(error: Error): ErrorWrapperState {
    return { error, info: null };
  }

  componentDidCatch(error: Error, errorInfo: React.ErrorInfo): void {
    this.setState({ info: errorInfo });
  }

  render(): React.ReactElement {
    if (this.state.error) {
      return (
        <Callout intent="danger" title="Error">
          <div className="text-white overflow-auto">
            Encountered error during rendering:
            <br />
            <pre>{this.state.error.stack ?? this.state.error.toString()}</pre>
            {this.state.info !== null && (
              <div>
                Component stack:
                <br />
                <pre>{this.state.info.componentStack}</pre>
              </div>
            )}
          </div>
        </Callout>
      );
    } else {
      return <div>{this.props.children}</div>;
    }
  }
}
