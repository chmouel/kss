# coding: utf-8
class Kss < Formula
  desc "Kubernetes pod status on steroid 💉"
  homepage "https://github.com/chmouel/kss"
  version "0.3.0"

  depends_on "fzf"
  depends_on "kubectl"

  url "https://github.com/chmouel/kss/tarball/#{version}"
  sha256 "91fdd89c4c690b43dd80f68ab255aa0dc81ad0f4cb3dcf26905f0484a426b8ea"

  def install
    bin.install "kss" => "kss"
    bin.install_symlink "kss" => "kubectl-kss"
    zsh_completion.install "_kss"
    prefix.install_metafiles
  end

end
