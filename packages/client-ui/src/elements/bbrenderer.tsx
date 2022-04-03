import { useMemo } from 'react';
import bbparser from '../lib/bbparser';
import { gs } from '../lib/state';

export default function BBRenderer(props: { content: string }): React.ReactElement {
  const html = useMemo(() => {
    return { __html: bbparser(props.content) };
  }, [props.content]);

  return <p onClick={listener} dangerouslySetInnerHTML={html} />;
}

function listener(e: React.MouseEvent) {
  if (e.target instanceof HTMLAnchorElement) {
    e.preventDefault();
    void gs.client.openLink({ link: e.target.href });
  }
}
