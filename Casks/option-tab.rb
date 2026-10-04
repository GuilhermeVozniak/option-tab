cask "option-tab" do
  version "0.6.3"
  sha256 "734364b26f471630c7c22738ee873128d1c78401c746f99000828ff2bea71a39"

  url "https://github.com/GuilhermeVozniak/option-tab/releases/download/v#{version}/option-tab_#{version}_darwin_universal.dmg"
  name "Option Tab"
  desc "Keyboard window switcher and native Dock enhancements"
  homepage "https://github.com/GuilhermeVozniak/option-tab"

  auto_updates true
  depends_on macos: :sonoma

  app "Option Tab.app"
end
