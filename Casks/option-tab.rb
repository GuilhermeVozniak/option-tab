cask "option-tab" do
  version "0.6.1"
  sha256 "bc0a20e51a91a06ce5cecd1626ec8f743c6eae13c71395b00433231bc3185f01"

  url "https://github.com/GuilhermeVozniak/option-tab/releases/download/v#{version}/option-tab_#{version}_darwin_universal.dmg"
  name "Option Tab"
  desc "Keyboard window switcher and native Dock enhancements"
  homepage "https://github.com/GuilhermeVozniak/option-tab"

  auto_updates true
  depends_on macos: :sonoma

  app "Option Tab.app"
end
