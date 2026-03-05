{
  description = "crd-schemas - Automated CRD JSON Schema extraction pipeline";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
  };

  outputs = { self, nixpkgs }:
    let
      systems = [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ];
      forAllSystems = f: nixpkgs.lib.genAttrs systems (system: f {
        pkgs = nixpkgs.legacyPackages.${system};
      });
    in
    {
      packages = forAllSystems ({ pkgs }: {
        default = pkgs.buildGoModule {
          pname = "crd-schemas";
          version = "0.1.0";
          src = ./.;
          vendorHash = "sha256-9+/SUDnd4cH35NMMSFKZiDrRu14xYyuzXPoxrHIRu4U=";
          subPackages = [ "cmd/extract" ];
        };
      });

      devShells = forAllSystems ({ pkgs }: {
        default = pkgs.mkShell {
          packages = [
            pkgs.go
            pkgs.kubernetes-helm
            pkgs.check-jsonschema
            pkgs.yq-go
          ];
        };
      });
    };
}
