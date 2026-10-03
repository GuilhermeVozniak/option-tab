cask "option-tab" do
  version "0.6.0"
  sha256 "f0257f33caf23104014d29fc597da407104803b76922ccb653e01dcd5b14273b"

  url "https://github.com/GuilhermeVozniak/option-tab/releases/download/v#{version}/option-tab_#{version}_darwin_universal.dmg"
  name "Option Tab"
  desc "Keyboard window switcher and native Dock enhancements"
  homepage "https://github.com/GuilhermeVozniak/option-tab"

  auto_updates true
  depends_on macos: :sonoma

  app "Option Tab.app"
end
