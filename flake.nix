{
  description = "browser-tools — let AI agents drive a Chrome browser";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs?ref=nixpkgs-unstable";
  };

  outputs =
    { self, nixpkgs, ... }:

    let
      forAllSystems =
        function:
        nixpkgs.lib.genAttrs [
          "x86_64-linux"
          "aarch64-linux"
          "x86_64-darwin"
          "aarch64-darwin"
        ] (system: function nixpkgs.legacyPackages.${system});

      version = if (self ? shortRev) then self.shortRev else "dev";
    in
    {
      devShells = forAllSystems (pkgs: {
        default = pkgs.mkShell {
          packages = [
            pkgs.go
            pkgs.golangci-lint
          ];
        };
      });

      packages = forAllSystems (pkgs: {
        browser-tools = pkgs.buildGoModule {
          pname = "browser-tools";
          inherit version;
          src = pkgs.lib.cleanSource self;

          # Update the hash if go dependencies change!
          # vendorHash = pkgs.lib.fakeHash;
          vendorHash = "sha256-ThIribgZId/u2eX6mlBuvZB99XHombSk6Gwr2XRmfUs=";

          env.GOFLAGS = "-trimpath";
          ldflags = [
            "-s"
            "-w"
          ];

          meta = {
            description = "Let AI agents drive a Chrome browser";
            platforms = pkgs.lib.platforms.darwin ++ pkgs.lib.platforms.linux;
            mainProgram = "browser-tools";
          };
        };

        browser-tools-skill = pkgs.runCommand "browser-tools-skill" { } ''
          mkdir -p $out/scripts
          cp ${./SKILL.md} $out/SKILL.md
          cp ${./REFERENCE.md} $out/REFERENCE.md
          ln -s ${
            self.packages.${pkgs.stdenv.hostPlatform.system}.browser-tools
          }/bin/browser-tools $out/scripts/browser-tools
        '';

        default = self.packages.${pkgs.stdenv.hostPlatform.system}.browser-tools;
      });
    };
}
