# Pawl Icons Extension for Zed Editor

Official Pawl file icons for [Zed](https://zed.dev).

## 📦 Installation

### Quick Install (Local Dev Extension):

1. Open **Zed Editor**.
2. Open the Command Palette (`Ctrl+Shift+P`).
3. Run `zed: extensions`.
4. Click **Install Dev Extension** in the top-right corner.
5. Select this folder (`editors/zed` or `C:\Users\geekom\Desktop\Pawl\editors\zed`).

### Enable the Icon Theme:

In your Zed `settings.json` (`Ctrl+,` or `~/.config/zed/settings.json` / `%APPDATA%\Zed\settings.json`):

```json
{
  "icon_theme": "Pawl Icons",
  "tabs": {
    "file_icons": true
  }
}
```
Or open the Command Palette (`Ctrl+Shift+P`) and run:
`icon theme selector: toggle` → select **Pawl Icons**.
