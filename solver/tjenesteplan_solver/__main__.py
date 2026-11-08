"""python -m tjenesteplan_solver            < inndata.json > utdata.json
python -m tjenesteplan_solver --check      som over, men svaret får «violations»: brudd funnet av den uavhengige sjekkeren
python -m tjenesteplan_solver check inndata.json utdata.json
"""

import json
import sys

from .check import check
from .model import solve


def main(argv: list[str]) -> int:
    if len(argv) == 4 and argv[1] == "check":
        with open(argv[2], encoding="utf-8") as f:
            data = json.load(f)
        with open(argv[3], encoding="utf-8") as f:
            result = json.load(f)
        problems = check(data, result)
        for p in problems:
            print(p)
        print("OK" if not problems else f"{len(problems)} brudd")
        return 1 if problems else 0
    data = json.load(sys.stdin)
    result = solve(data)
    if len(argv) == 2 and argv[1] == "--check":
        result["violations"] = check(data, result)
    json.dump(result, sys.stdout, ensure_ascii=False, indent=1)
    sys.stdout.write("\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
