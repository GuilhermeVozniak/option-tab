cask "option-tab" do
  version "0.4.8"
  sha256 "5ae22160923c74767d6a381bf8aac56d7749162eca435fc4457cbc8036ed1186"

  url "https://github.com/GuilhermeVozniak/option-tab/releases/download/v#{version}/option-tab_#{version}_darwin_arm64.dmg"
  name "Option Tab"
  desc "Keyboard window switcher and native Dock enhancements"
  homepage "https://github.com/GuilhermeVozniak/option-tab"

  auto_updates true
  depends_on arch: :arm64
  depends_on macos: :sonoma

  app "option-tab.app"
end
