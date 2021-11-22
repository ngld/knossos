#import <Cocoa/Cocoa.h>
#include <libgen.h>
#include "platform.h"

void PlatformInit() {}

void ShowError(const char *message) {
  NSString *text = [[NSString alloc] initWithUTF8String:message];

  NSAlert *alert = [[NSAlert alloc] init];
  [alert addButtonWithTitle:@"OK"];
  [alert setMessageText:text];
  [alert setAlertStyle:NSAlertStyleCritical];
  [alert runModal];
}

static DialogResult RunSavePanel(NSSavePanel *panel,
                         const char *title, const char *default_filepath) {
  panel.title = [NSString stringWithUTF8String:title];
  // panel.message = [NSString stringWithUTF8String:message];

  panel.nameFieldStringValue =
      [NSString stringWithUTF8String:basename((char*)default_filepath)];
  panel.directoryURL =
      [NSURL fileURLWithPath:[NSString stringWithUTF8String:dirname((char*)default_filepath)]
                 isDirectory:YES];

  bool success = [panel runModal] == NSModalResponseOK;

  DialogResult result;
  if (success) {
    if (panel.URL != nil) {
      const char *path = panel.URL.path.UTF8String;
      int length = strlen(path);
      result.string = malloc((length + 1) * sizeof(char));
      strncpy(result.string, path, length + 1);
    }

    result.code = 0;
  } else {
    result.code = 1;
  }
  return result;
}

DialogResult SaveFileDialog(const char *title, const char *default_filepath) {
  NSSavePanel *panel = [NSSavePanel savePanel];
  panel.prompt = @"Save";
  return RunSavePanel(panel, title, default_filepath);
}

DialogResult OpenFileDialog(const char *title, const char *default_filepath) {
  NSOpenPanel *panel = [NSOpenPanel openPanel];
  panel.canChooseFiles = YES;
  panel.canChooseDirectories = NO;
  panel.allowsMultipleSelection = NO;
  panel.prompt = @"Open";
  return RunSavePanel(panel, title, default_filepath);
}

DialogResult OpenFolderDialog(const char *title, const char *folder) {
  NSOpenPanel *panel = [NSOpenPanel openPanel];
  panel.canChooseFiles = NO;
  panel.canChooseDirectories = YES;
  panel.allowsMultipleSelection = NO;
  panel.prompt = @"Open Folder";
  return RunSavePanel(panel, title, folder);
}

extern char* GetDesktopDirectory() {
  char* home_path = getenv("HOME");
  char* path = malloc(sizeof(char) * (strlen(home_path) + 9));

  strcpy(path, home_path);
  strcat(path, "/Desktop");
  return path;
}

extern char* GetStartMenuDirectory() {
  return NULL;
}

extern bool IsElevated() {
  return false;
}

extern const char* CreateShortcut(const char* shortcut, const char* target) {
  const char* error = "unsupported platform";
  int errlen = sizeof(char) * (strlen(error) + 1);
  char* result = (char*)malloc(errlen);
  memcpy(result, error, errlen);
  return result;
}

extern char *RunElevated(const char *program, const char *args) {
  const char* error = "unsupported platform";
  int errlen = sizeof(char) * (strlen(error) + 1);
  char* result = (char*)malloc(errlen);
  memcpy(result, error, errlen);
  return result;
}
