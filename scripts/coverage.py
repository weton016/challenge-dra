import json, pathlib
root=pathlib.Path(__file__).resolve().parents[1]
blocks={}
for line in (root/'reports/coverage.out').read_text().splitlines()[1:]:
 location,statements,count=line.split()
 if '/internal/' not in location: continue
 old=blocks.get(location,(int(statements),0))
 blocks[location]=(int(statements),max(old[1],int(count)))
def percent(items):
 items=list(items)
 total=sum(n for n,c in items)
 return 100*sum(n for n,c in items if c>0)/total if total else 0
overall=percent(blocks.values())
domain=percent(v for k,v in blocks.items() if '/internal/ledger/' in k)
report={'internal_statement_coverage':overall,'ledger_statement_coverage':domain,'threshold_internal':85,'threshold_ledger':100}
(root/'reports/coverage.json').write_text(json.dumps(report,indent=2))
print(json.dumps(report))
raise SystemExit(0 if overall>=85 and domain==100 else 1)
