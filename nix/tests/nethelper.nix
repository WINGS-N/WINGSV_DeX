{ nixpkgs, module, package }:

let
  system = "x86_64-linux";
  pkgs = import nixpkgs { inherit system; };
in
{
  name = "wingsv-dex-nethelper";
  nodes.machine = { lib, ... }: {
    imports = [ module ];

    services.wingsv-dex = {
      enable = true;
      user = "tester";
      privilegedHelper.enable = true;
    };
    environment.systemPackages = [ pkgs.python3 pkgs.iproute2 ];

    users.users.tester = {
      isNormalUser = true;
      uid = 1000;
    };
    users.users.other = {
      isNormalUser = true;
      uid = 1001;
    };

    # Keep this a headless, minimal VM; the helper must not depend on a GUI.
    services.xserver.enable = lib.mkForce false;
    virtualisation.graphics = false;
    system.stateVersion = "24.11";
  };

  testScript = ''
    import shlex
    import json

    def as_user(user, code):
        command = "python3 -c " + shlex.quote(code)
        return "su -s /bin/sh " + user + " -c " + shlex.quote(command)

    client = "import socket,json; s=socket.socket(socket.AF_UNIX); s.connect('/run/wingsv-dex/nethelper.sock'); s.sendall(json.dumps({'cmd':'wat'}).encode()+bytes([10])); print(s.makefile().readline())"

    machine.start()
    machine.wait_for_unit("wingsv-dex-nethelper.service")
    machine.succeed("systemctl is-active --quiet wingsv-dex-nethelper.service")
    machine.wait_until_succeeds("test -S /run/wingsv-dex/nethelper.sock")
    machine.succeed("test \"$(stat -c '%a:%U' /run/wingsv-dex/nethelper.sock)\" = '600:tester'")

    # The malformed/unknown command must be rejected without running network actions.
    before = machine.succeed("ip -o route show")
    response = machine.succeed(as_user("tester", client))
    after = machine.succeed("ip -o route show")
    assert before == after, "unknown command mutated routes"
    assert json.loads(response).get("error"), response

    # A different uid cannot open the tester-owned 0600 socket.
    denied = "import socket; s=socket.socket(socket.AF_UNIX); s.connect('/run/wingsv-dex/nethelper.sock')"
    machine.fail(as_user("other", denied))
    machine.succeed("systemctl is-active --quiet wingsv-dex-nethelper.service")

    # Reconnect after the first client closed its connection.
    response = machine.succeed(as_user("tester", client))
    assert json.loads(response).get("error"), response
    machine.succeed("systemctl is-active --quiet wingsv-dex-nethelper.service")

    # Exercise privileged setup and teardown only inside the disposable VM.
    start_client = "import socket,json; s=socket.socket(socket.AF_UNIX); s.connect('/run/wingsv-dex/nethelper.sock'); s.sendall(json.dumps({'cmd':'start','protectSocket':'wingsv-dex-protect','fwmark':34952,'tunName':'wingsv-tun'}).encode()+bytes([10])); print(s.makefile().readline()); s.sendall(json.dumps({'cmd':'stop'}).encode()+bytes([10])); s.close()"
    before = machine.succeed("ip -o route show")
    response = machine.succeed(as_user("tester", start_client))
    assert json.loads(response).get("ok"), response
    assert machine.succeed("ip -o route show") == before
    machine.succeed("systemctl is-active --quiet wingsv-dex-nethelper.service")

    # The Xray TUN child must run as tester, retaining only network authority.
    xray_client = """
    import json, socket, subprocess, time
    s = socket.socket(socket.AF_UNIX)
    s.connect('/run/wingsv-dex/nethelper.sock')
    def send(command):
        s.sendall(json.dumps(command).encode() + bytes([10]))
        return json.loads(s.makefile().readline())
    assert send({'cmd':'start','protectSocket':'wingsv-dex-protect','fwmark':34952,'tunName':'wingsv-tun'}).get('ok')
    config = {'log':{'loglevel':'warning'},'inbounds':[{'tag':'tun-in','protocol':'tun','settings':{'name':'wingsv-tun','mtu':1500,'autoOutboundsInterface':'eth1'}}],'outbounds':[{'tag':'proxy','protocol':'freedom','settings':{}}]}
    result = send({'cmd':'xrayup','xrayBin':'${package}/bin/xray','xrayConfig':json.dumps(config),'tunName':'wingsv-tun'})
    assert result.get('ok'), result
    time.sleep(2)
    pid = int(subprocess.check_output(['pgrep','-u','tester','-x','xray']).splitlines()[0])
    status = open('/proc/%d/status' % pid).read()
    uid = next(line for line in status.splitlines() if line.startswith('Uid:')).split()
    cap = next(line for line in status.splitlines() if line.startswith('CapEff:')).split()[1]
    ambient = next(line for line in status.splitlines() if line.startswith('CapAmb:')).split()[1]
    assert uid[1:3] == ['1000','1000'], uid
    assert int(cap, 16) == 1 << 12, cap
    assert int(ambient, 16) == 1 << 12, ambient
    assert send({'cmd':'xraydown'}).get('ok')
    s.sendall(json.dumps({'cmd':'stop'}).encode() + bytes([10]))
    s.close()
    """
    machine.succeed(as_user("tester", xray_client))
  '';
}
