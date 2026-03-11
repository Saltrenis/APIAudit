class Apiaudit < Formula
  desc "Zero-config API route scanner and auditor"
  homepage "https://github.com/Saltrenis/APIAudit"
  license "MIT"
  head "https://github.com/Saltrenis/APIAudit.git", branch: "main"

  # TODO: Once releases exist, add:
  # url "https://github.com/Saltrenis/APIAudit/archive/refs/tags/v#{version}.tar.gz"
  # sha256 "..."

  depends_on "go" => :build

  def install
    system "go", "build", *std_go_args(ldflags: "-s -w"), "./cmd/apiaudit"
  end

  test do
    assert_match "apiaudit", shell_output("#{bin}/apiaudit --help")
  end
end
