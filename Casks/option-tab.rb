cask "option-tab" do
  version "0.6.2"
  sha256 "262947b4faa1805be7eb3c731476bc3464c01c99c1169e4a75a57e294ff47165"

  url "https://github.com/GuilhermeVozniak/option-tab/releases/download/v#{version}/option-tab_#{version}_darwin_universal.dmg"
  name "Option Tab"
  desc "Keyboard window switcher and native Dock enhancements"
  homepage "https://github.com/GuilhermeVozniak/option-tab"

  auto_updates true
  depends_on macos: :sonoma

  app "Option Tab.app"
end
