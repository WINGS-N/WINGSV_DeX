{ package }:
{ config, lib, ... }:

let
  cfg = config.services.wingsv-dex;
in
{
  options.services.wingsv-dex = {
    enable = lib.mkEnableOption "WINGS V DeX";
    user = lib.mkOption {
      type = lib.types.str;
      default = "";
      description = "User account that runs WINGS V DeX.";
    };
    package = lib.mkOption {
      type = lib.types.package;
      default = package;
      defaultText = lib.literalExpression "self.packages.x86_64-linux.default";
      description = "WINGS V DeX package to run.";
    };
  };

  config = lib.mkIf cfg.enable {
    assertions = [
      {
        assertion = cfg.user != "";
        message = "services.wingsv-dex.user must be set to an explicit username.";
      }
    ];

    environment.systemPackages = [ cfg.package ];
    security.polkit.enable = true;

    systemd.user.services.wingsv-dex = {
      description = "WINGS V DeX";
      wantedBy = [ "graphical-session.target" ];
      after = [ "graphical-session.target" ];
      partOf = [ "graphical-session.target" ];
      unitConfig.ConditionUser = cfg.user;
      serviceConfig = {
        Type = "simple";
        ExecStart = "${cfg.package}/bin/wingsv-dex --autostart";
        Restart = "on-failure";
      };
    };
  };
}
