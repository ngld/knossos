import { FileRef } from '@api/mod';
import { API_URL } from '../lib/constants';

export interface RefImageProps extends React.HTMLAttributes<HTMLImageElement> {
  src?: FileRef;
}
export default function RefImage(props: RefImageProps): React.ReactElement | null {
  return props.src ? (
    <img
      {...props}
      src={props.src.urls.length > 0 ? props.src.urls[0] : API_URL + '/ref/' + props.src.fileid}
    />
  ) : null;
}
