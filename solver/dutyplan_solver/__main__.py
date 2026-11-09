"""python -m dutyplan_solver            < input.json > output.json
python -m dutyplan_solver --check      as above, but the answer gets «violations»: violations found by the independent checker
python -m dutyplan_solver check input.json output.json
python -m dutyplan_solver --defaults   prints the default parameters (the weights and so on) as JSON
"""

import json
import sys

from .check import check
from .model import DEFAULT_PARAMS, solve


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
    if argv[1:] == ["--defaults"]:
        json.dump(DEFAULT_PARAMS, sys.stdout, ensure_ascii=False, indent=1)
        sys.stdout.write("\n")
        return 0
    data = json.load(sys.stdin)
    result = solve(data)
    if len(argv) == 2 and argv[1] == "--check":
        result["violations"] = check(data, result)
    json.dump(result, sys.stdout, ensure_ascii=False, indent=1)
    sys.stdout.write("\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
