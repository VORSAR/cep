import sys
bad = []
for spec in sys.argv[1:]:
    path, name = spec.rsplit(':', 1)
    src = open(path, encoding='utf-8').read().split('\n')
    i = next(k for k, l in enumerate(src) if l.startswith(f'func {name}('))
    j, doc = i - 1, []
    while j >= 0 and src[j].startswith('//'):
        doc.append(src[j]); j -= 1
    if src[i + 1].strip() != 't.Parallel()' and not any('Not parallel:' in d for d in doc):
        bad.append(spec)
print(len(bad), 'without t.Parallel or a reason:', bad); sys.exit(1 if bad else 0)
