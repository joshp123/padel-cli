{
  description = "Padel court booking CLI";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in {
        packages.default = pkgs.buildGoModule {
          pname = "padel-cli";
          version = "0.1.0";
          src = ./.;
          go = pkgs.go_1_25;
          vendorHash = "sha256-lfET2hIQtnxdG8byLFFIfPWwty9/giml2DzSzow8H60=";
          postInstall = ''
            ln -s $out/bin/padel-cli $out/bin/padel
          '';
        };

        devShells.default = pkgs.mkShell {
          buildInputs = [ pkgs.go pkgs.gopls ];
        };
      }
    ) // {
      # Top-level openclawPlugin for nix-openclaw
      openclawPlugin = let
        system = builtins.currentSystem;
      in {
        name = "padel";
        skills = [ ./skills/padel ];
        packages = [ self.packages.${system}.default ];
        needs = {
          stateDirs = [ ".config/padel" ];
          requiredEnv = [ "PADEL_AUTH_FILE" ];
        };
      };
    };
}
