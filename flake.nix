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
        };

        devShells.default = pkgs.mkShell {
          buildInputs = [ pkgs.go pkgs.gopls ];
        };
      });
}
