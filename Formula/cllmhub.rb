class Cllmhub < Formula
  desc "Turn your local LLM into a production API"
  homepage "https://github.com/cllmhub/cllmhub-cli"
  version "0.5.7"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/cllmhub/cllmhub-cli/releases/download/v#{version}/cllmhub-darwin-arm64"
      sha256 "cd71228097d54194f5bb9f3ae01b0865f032239dde7c1bfaff09628dbfe96b97"
    else
      url "https://github.com/cllmhub/cllmhub-cli/releases/download/v#{version}/cllmhub-darwin-amd64"
      sha256 "cd71228097d54194f5bb9f3ae01b0865f032239dde7c1bfaff09628dbfe96b97"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/cllmhub/cllmhub-cli/releases/download/v#{version}/cllmhub-linux-arm64"
      sha256 "cd71228097d54194f5bb9f3ae01b0865f032239dde7c1bfaff09628dbfe96b97"
    else
      url "https://github.com/cllmhub/cllmhub-cli/releases/download/v#{version}/cllmhub-linux-amd64"
      sha256 "cd71228097d54194f5bb9f3ae01b0865f032239dde7c1bfaff09628dbfe96b97"
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
