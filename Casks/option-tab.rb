cask "option-tab" do
  version "0.6.4"
  sha256 "79786e6c67c3db7dfce32a3931fa483ef3e0563f94f78ebd89d077a16927ea9e"

  url "https://github.com/GuilhermeVozniak/option-tab/releases/download/v#{version}/option-tab_#{version}_darwin_universal.dmg"
  name "Option Tab"
  desc "Keyboard window switcher and native Dock enhancements"
  homepage "https://github.com/GuilhermeVozniak/option-tab"

  auto_updates true
  depends_on macos: :sonoma

  app "Option Tab.app"
end
