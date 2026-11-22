"""B-del's covered-set check (G2-95; DQ-S3): the covered blocks of each profile, as `awk 'NR>1 && $NF>0 {print $1}' | sort -u`."""
import sys, os
d = sys.argv[1]
def cov(name):
    with open(os.path.join(d, name + '.out'), encoding='utf-8') as f:
        lines = f.read().splitlines()[1:]
    return {l.split()[0] for l in lines if l.split()[-1] != '0'}
s = {n: cov(n) for n in ('old1', 'old2', 'new1', 'new2', 'deleted', 'init')}
for n, v in s.items():
    print(f'{n}: {len(v)} covered blocks')
inter = s['new1'] & s['new2']
print('deleted test subset of new1 & new2:', s['deleted'] <= inter, '| missing:', sorted(s['deleted'] - inter))
print('deleted test == init only (-run ^$):', s['deleted'] == s['init'], '| extra over init:', sorted(s['deleted'] - s['init']))
eq = s['old1'] == s['old2'] == s['new1'] == s['new2']
print('equality across all four runs:', eq)
for a, b in (('old1', 'old2'), ('new1', 'new2'), ('old1', 'new1'), ('old2', 'new2'), ('old1', 'new2'), ('old2', 'new1')):
    print(f'  {a} vs {b}: only in {a} {sorted(s[a] - s[b])}; only in {b} {sorted(s[b] - s[a])}')
