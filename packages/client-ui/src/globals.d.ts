type MessageReceiver = (msg: ArrayBuffer) => void;
declare function knShowDevTools(): void;
declare function knAddMessageListener(cb: MessageReceiver): void;
declare function knRemoveMessageListener(cb: MessageReceiver): void;
declare function knOpenFile(
  title: string,
  default_filename: string,
  accepted: string[],
): Promise<string>;
declare function knOpenFolder(title: string, default_folder: string): Promise<string>;
declare function knSaveFile(
  title: string,
  default_filename: string,
  accepted: string[],
): Promise<string>;
declare function knMinimizeWindow(): void;
declare function knRestoreWindow(): void;
declare function knMaximizeWindow(): void;
declare function knCloseWindow(): void;

declare module '*.jpg' {
  const url: string;
  export default url;
}
