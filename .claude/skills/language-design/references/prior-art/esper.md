# Prior art: Esper EPL

*Read when:* you need vocabulary for consumption/multiplicity policies or `every` semantics (Phase 2, Phase 3 patterns).

## 2. Esper EPL

Shape: SQL dialect with a pattern sub-language in brackets; also supports `match_recognize`.

```
select a.custId, sum(a.price + b.price)
from pattern [every a=ServiceOrder -> b=ProductOrder(custId = a.custId) where timer:within(1 min)].win:time(2 hour)
where a.name in ('Repair', b.name)
group by a.custId having sum(a.price + b.price) > 100;

select * from pattern @DiscardPartialsOnMatch [every a=A -> B and C(id=a.id)];
```

Pattern operators (verified, Esper 6–9): `every`, `every-distinct`, `[num]` repeat, `until`, `and`, `or`, `not`, `->` followed-by; guards `timer:within`, `timer:withinmax`, `while`; observers `timer:interval`, `timer:at` (crontab), `timer:schedule`; pattern-level annotations `@SuppressOverlappingMatches`, `@DiscardPartialsOnMatch` for match multiplicity; `match_recognize` as an alternative with a documented comparison table against EPL patterns.

Gets right: the most complete vocabulary for consumption/multiplicity policy (the annotations), `every-distinct`, `timer:withinmax` (count-bounded `within`), contexts (`create context … partition by`), named windows and tables with clear semantics.

Gets wrong: pattern language is nested and bracket-heavy; `every` placement changes meaning subtly; the `timer:` namespace is function-call noise for readers; two overlapping ways to say patterns; processing-time by default.

Inherit: the *concepts* of suppress-overlapping / discard-partials as named policies; `every-distinct`; count-bounded within; context/partition as a separate declaration.
