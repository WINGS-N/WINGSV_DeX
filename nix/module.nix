{ package }:
{ config, lib, pkgs, ... }:

let
  cfg = config.services.wingsv-dex;
  helperStart = pkgs.writeShellScript "wingsv-dex-nethelper-start" ''
    exec ${cfg.package}/bin/.wingsv-dex-wrapped --net-helper-server /run/wingsv-dex/nethelper.sock "$(${pkgs.coreutils}/bin/id -u ${lib.escapeShellArg cfg.user})"
  '';
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
    privilegedHelper.enable = lib.mkEnableOption "passwordless access to the privileged network helper (grants the selected user host network administration)";
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
        Environment = lib.mkIf cfg.privilegedHelper.enable [
          "WINGSV_NET_HELPER_SOCKET=/run/wingsv-dex/nethelper.sock"
        ];
      };
    };

    systemd.services.wingsv-dex-nethelper = lib.mkIf cfg.privilegedHelper.enable {
      description = "WINGS V DeX privileged network helper";
      wantedBy = [ "multi-user.target" ];
      after = [ "local-fs.target" ];
      path = with pkgs; [ coreutils nftables kmod iproute2 amneziawg-tools ];
      serviceConfig = {
        Type = "simple";
        User = "root";
        Group = "root";
        RuntimeDirectory = "wingsv-dex";
        RuntimeDirectoryMode = "0755";
        ExecStart =
          if builtins.hasAttr cfg.user config.users.users && config.users.users.${cfg.user}.uid != null then
            "${cfg.package}/bin/.wingsv-dex-wrapped --net-helper-server /run/wingsv-dex/nethelper.sock ${toString config.users.users.${cfg.user}.uid}"
          else
            "${helperStart}";
        Restart = "on-failure";
        ProtectSystem = "strict";
        ProtectHome = "read-only";
        PrivateTmp = true;
        PrivateDevices = false;
        # WireGuard setup writes interface-specific network sysctls under /proc/sys.
        ProtectKernelTunables = false;
        ProtectKernelModules = false;
        ProtectControlGroups = false;
        RestrictAddressFamilies = [ "AF_UNIX" "AF_INET" "AF_INET6" "AF_NETLINK" "AF_PACKET" ];
      };
    };
  };
}
