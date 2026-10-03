cask "option-tab" do
  version "0.5.0"
  sha256 "6ef2d41f3e1593bafb9d79c570e8e5e7c2eef1e5b05457584ab2f9d1b1e4e231"

  url "https://github.com/GuilhermeVozniak/option-tab/releases/download/v#{version}/option-tab_#{version}_darwin_universal.dmg"
  name "Option Tab"
  desc "Keyboard window switcher and native Dock enhancements"
  homepage "https://github.com/GuilhermeVozniak/option-tab"

  auto_updates true
  depends_on macos: :sonoma

  app "Option Tab.app"
end
