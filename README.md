# promrelabel

Tool for easier relabeling prometheus TSDB block data. This cli tool does the same thing
as `thanos tools bucket rewrite --rewrite.to-relabel-config`, but with support for Prometheus TSDB data only.

```shell
usage: promrelabel [<flags>] [<db path>]

A block storage based long-term storage for Prometheus.

Flags:
  -h, --help                 Show context-sensitive help (also try --help-long and --help-man).
      --relabel-config=RELABEL-CONFIG  
                             Relabel configuration file path
      --id=ID ...            Block IDs to apply relabeling
      --dry-run              Dry run mode
      --delete-source-block  Delete source block

Args:
  [<db path>]  Database path (default is data/).

```

Example usage:

``` shell
promrelabel --relabel-config relabel.yaml --id 01FET1EK9BC3E0QD4886RQCM8K --id 01FGDRJNST2EYDY2RKWFZJPGWJ  --no-dry-run .

level=info msg="changelog will be available" file=01FGEC4TQZP7GBKJYSQ8XGN7XH/change.log
level=info msg="starting rewrite for block" source=01FET1EK9BC3E0QD4886RQCM8K new=01FGEC4TQZP7GBKJYSQ8XGN7XH toRelabel="- action: drop\n  regex: k8s_app_metric37\n  source_labels: [__name__]\n- action: replace\n  source_labels: [__name__]\n  regex: k8s_app_metric38\n  target_label: __name__\n  replacement: old_metric\n"
level=info msg="processed 10.00% of 14700 series"
level=info msg="processed 20.00% of 14700 series"
level=info msg="processed 30.00% of 14700 series"
level=info msg="processed 40.00% of 14700 series"
level=info msg="processed 50.00% of 14700 series"
level=info msg="processed 60.00% of 14700 series"
level=info msg="processed 70.00% of 14700 series"
level=info msg="processed 80.00% of 14700 series"
level=info msg="processed 90.00% of 14700 series"
level=info msg="processed 100.00% of 14700 series"
level=info msg="wrote new block after modifications; flushing" source=01FET1EK9BC3E0QD4886RQCM8K new=01FGEC4TQZP7GBKJYSQ8XGN7XH
level=info msg="created new block" source=01FET1EK9BC3E0QD4886RQCM8K new=01FGEC4TQZP7GBKJYSQ8XGN7XH
level=info msg="changelog will be available" file=01FGEC4VSPBNBZEC3RP93FJFWY/change.log
level=info msg="starting rewrite for block" source=01FGDRJNST2EYDY2RKWFZJPGWJ new=01FGEC4VSPBNBZEC3RP93FJFWY toRelabel="- action: drop\n  regex: k8s_app_metric37\n  source_labels: [__name__]\n- action: replace\n  source_labels: [__name__]\n  regex: k8s_app_metric38\n  target_label: __name__\n  replacement: old_metric\n"
level=info msg="processed 10.00% of 14700 series"
level=info msg="processed 20.00% of 14700 series"
level=info msg="processed 30.00% of 14700 series"
level=info msg="processed 40.00% of 14700 series"
level=info msg="processed 50.00% of 14700 series"
level=info msg="processed 60.00% of 14700 series"
level=info msg="processed 70.00% of 14700 series"
level=info msg="processed 80.00% of 14700 series"
level=info msg="processed 90.00% of 14700 series"
level=info msg="processed 100.00% of 14700 series"
level=info msg="wrote new block after modifications; flushing" source=01FGDRJNST2EYDY2RKWFZJPGWJ new=01FGEC4VSPBNBZEC3RP93FJFWY
level=info msg="created new block" source=01FGDRJNST2EYDY2RKWFZJPGWJ new=01FGEC4VSPBNBZEC3RP93FJFWY
```

# Note

Tombstone file is ignored while relabeling.
