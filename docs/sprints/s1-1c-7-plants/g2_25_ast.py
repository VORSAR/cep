"""G2-25's K2e check as the record describes it: every subprocess.run passes timeout=, every go call passes env=."""
import ast, sys

def main(path):
    tree = ast.parse(open(path, encoding="utf-8").read())
    faults = []
    for n in ast.walk(tree):
        if not isinstance(n, ast.Call):
            continue
        f = n.func
        if not (isinstance(f, ast.Attribute) and f.attr == "run" and isinstance(f.value, ast.Name) and f.value.id == "subprocess"):
            continue
        kws = {k.arg for k in n.keywords}
        if "timeout" not in kws:
            faults.append(f"{n.lineno}: subprocess.run without timeout=")
        argv = n.args[0] if n.args else None
        if isinstance(argv, ast.List) and argv.elts and isinstance(argv.elts[0], ast.Constant) and argv.elts[0].value == "go":
            if "env" not in kws:
                faults.append(f"{n.lineno}: go call without env=")
    for x in faults:
        print(x)
    print(f"faults: {len(faults)}")

main(sys.argv[1])
