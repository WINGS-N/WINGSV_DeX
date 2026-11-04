{ pkgs, src, goModules }:
let
  frontendSrc = src + "/frontend";
  wailsVersion = "3.1.0";
  wails = (pkgs.buildGoModule.override { go = pkgs.go_1_27; }) {
    pname = "wails3-cli";
    version = wailsVersion;
    src = pkgs.fetchFromGitHub {
      owner = "wailsapp";
      repo = "wails";
      rev = "v${wailsVersion}";
      hash = "sha256-Eqi2QEOdgjeJXYRtFJSYgqovDXn8PFoF5QExwhFg/+8=";
    };
    vendorHash = "sha256-VmWjstXfAcbNf/3XpP6pUPbq5K3EApSCANFVzKsruVE=";
    modRoot = "v3";
    subPackages = [ "cmd/wails3" ];
    nativeBuildInputs = [ pkgs.pkg-config ];
    buildInputs = [ pkgs.gtk4 pkgs.webkitgtk_6_0 pkgs.libsoup_3 ];
    doCheck = false;
  };
in
pkgs.stdenvNoCC.mkDerivation {
  pname = "wingsv-dex-frontend";
  version = "0.0.0";
  inherit src;
  nativeBuildInputs = [ pkgs.nodejs pkgs.pnpm pkgs.pnpmConfigHook pkgs.go_1_27 pkgs.pkg-config pkgs.stdenv.cc wails ];
  buildInputs = [ pkgs.gtk4 pkgs.webkitgtk_6_0 pkgs.libsoup_3 ];
  postPatch = ''
    mkdir -p frontend/dist
    touch frontend/dist/index.html
  '';
  preConfigure = "cd frontend";
  pnpmDeps = pkgs.fetchPnpmDeps {
    inherit (pkgs) stdenv;
    pname = "wingsv-dex-frontend";
    version = "0.0.0";
    src = frontendSrc;
    fetcherVersion = 5;
    hash = "sha256-oZTqfup+huK8RDhZAFjpzcD5KkqLHaMdctgJuTrrHa0=";
  };
  buildPhase = ''
    runHook preBuild
    cd ..
    export GOPROXY="file://${goModules}"
    export GOSUMDB=off
    export GOTOOLCHAIN=local
    export GOFLAGS=-buildvcs=false
    wails3 generate bindings -clean
    cd frontend
    pnpm run build
    runHook postBuild
  '';
  installPhase = ''
    mkdir -p $out/dist
    cp -r dist/. $out/dist/
  '';
}
