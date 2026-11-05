{ pkgs, src, ... }:

let
  buildGoModule = pkgs.buildGoModule.override { go = pkgs.go_1_27; };
  vkturnSrc = pkgs.fetchFromGitHub {
    owner = "WINGS-N";
    repo = "vk-turn-proxy";
    rev = "595e929935b8ee2415e2aac8cd77c7c5f19d2277";
    hash = "sha256-o6Qf7q6hB9yJNpfqGTyrqOM+m4LzcM9PhWkHIwtFeac=";
  };
  xraySrc = pkgs.fetchFromGitHub {
    owner = "WINGS-N";
    repo = "Xray-core";
    rev = "ad49d2f5765dd0ac845131eb3b3d35ebdcc10777";
    hash = "sha256-1THqBclpltuQ5DdvmRLR+S07L/OxhY8+mcszBmSPCmk=";
  };
  libXraySrc = pkgs.fetchFromGitHub {
    owner = "WINGS-N";
    repo = "libXray";
    rev = "d80d672bc77740453a348d2b2317950a9f08d1e5";
    hash = "sha256-JGA0w7gxnq8K2W1KXAagOo93Vyjs++ZvfFnviOPqiNo=";
  };
  byedpiSrc = pkgs.fetchFromGitHub {
    owner = "WINGS-N";
    repo = "byedpi";
    rev = "7f460e340d6dfc5ecb82beed1fc6883ae9a7a9c3";
    hash = "sha256-cnNcNjXgqwLn8fVxZZwy1V2vzibxmIIY7H91KcGLw6I=";
  };
in
{
  vkturn = buildGoModule {
    pname = "vkturn";
    version = "3.1.0";
    src = vkturnSrc;
    vendorHash = "sha256-Zm5JErJ+YHa5xiprjI+2HkTfN8YcLFZr0zEa3AWZaLw=";
    subPackages = [ "client" ];
    postInstall = ''
      mv "$out/bin/client" "$out/bin/vkturn"
    '';
  };

  xray = buildGoModule {
    pname = "xray";
    version = "unstable-2026-11-04";
    src = pkgs.runCommand "xrayhelper-source" {} ''
      mkdir -p $out/cmd
      cp -r ${src}/cmd/xrayhelper $out/cmd/xrayhelper
      chmod -R u+w $out/cmd/xrayhelper
      mkdir -p $out/external
      cp -r ${libXraySrc} $out/external/libXray
      cp -r ${xraySrc} $out/external/Xray-core
      chmod -R u+w $out/external
    '';
    modRoot = "cmd/xrayhelper";
    vendorHash = "sha256-UOygTWPfMVzxvXticg1jnwiZkQz8lHWQNnxKdeuPkKU=";
    subPackages = [ "." ];
    postInstall = ''
      mv "$out/bin/xrayhelper" "$out/bin/xray"
    '';
  };

  byedpi = pkgs.stdenv.mkDerivation {
    pname = "byedpi";
    version = "unstable-2026-11-04";
    src = byedpiSrc;
    nativeBuildInputs = [ pkgs.gnumake pkgs.gcc ];
    buildPhase = "make";
    installPhase = ''
      mkdir -p "$out/bin"
      install -m755 ciadpi "$out/bin/byedpi"
    '';
  };
}
