import argparse
import json
import sys


def main():
    parser = argparse.ArgumentParser(description="识别中国版欧姆龙 HEM-1026 的屏幕读数")
    parser.add_argument("photo")
    parser.add_argument("--debug-dir", metavar="DIR")
    args = parser.parse_args()
    try:
        from .pipeline import FIELDS, read_photo

        result = read_photo(args.photo, args.debug_dir)
    except (ValueError, OSError) as exc:
        parser.exit(2, f"{exc}\n")
    print(json.dumps(result, ensure_ascii=False))
    return int(any(result[name] is None for name in FIELDS))


if __name__ == "__main__":
    sys.exit(main())
