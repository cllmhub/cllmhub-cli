class Cllmhub < Formula
  desc "Turn your local LLM into a production API"
  homepage "https://github.com/cllmhub/cllmhub-cli"
  version "0.5.12"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/cllmhub/cllmhub-cli/releases/download/v#{version}/cllmhub-darwin-arm64"
      sha256 "2c2298a73ac6c077ea7c396bb9cac8ca963e162d98c614dbd252defd0cb56ff7"
    else
      url "https://github.com/cllmhub/cllmhub-cli/releases/download/v#{version}/cllmhub-darwin-amd64"
      sha256 "2c2298a73ac6c077ea7c396bb9cac8ca963e162d98c614dbd252defd0cb56ff7"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/cllmhub/cllmhub-cli/releases/download/v#{version}/cllmhub-linux-arm64"
      sha256 "2c2298a73ac6c077ea7c396bb9cac8ca963e162d98c614dbd252defd0cb56ff7"
    else
      url "https://github.com/cllmhub/cllmhub-cli/releases/download/v#{version}/cllmhub-linux-amd64"
      sha256 "2c2298a73ac6c077ea7c396bb9cac8ca963e162d98c614dbd252defd0cb56ff7"
    end
  end

  def install
    binary = Dir.glob("cllmhub-*").first || "cllmhub"
    bin.install binary => "cllmhub"
  end

  test do
    assert_match "cllmhub", shell_output("#{bin}/cllmhub --version")
  end
end
