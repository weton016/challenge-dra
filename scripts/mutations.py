"""Targeted semantic mutations of financial rules; never edits the worktree.

Each listed mutation must compile and be killed by real tests. This selected
suite is deliberately reported separately from exhaustive mutation coverage.
"""
import json, os, pathlib, shutil, subprocess, tempfile

root = pathlib.Path(__file__).resolve().parents[1]
go = os.environ.get('GO', 'go')
mutants = [
 ('allow-overdraft', 'left.Sign() < 0', 'left.Sign() < -1'),
 ('reject-exact-balance', 'left.Sign() < 0', 'left.Sign() <= 0'),
 ('miss-overflow', '!left.IsInt64()', 'false'),
 ('ignore-reserved', 'left.Sub(left, debitsPending)', '_ = debitsPending'),
 ('add-posted-debits', 'Sub(creditsPosted, debitsPosted)', 'Add(creditsPosted, debitsPosted)'),
 ('posted-debit-credits', 'case Posted:\n\t\tif debit {\n\t\t\treturn -amount', 'case Posted:\n\t\tif debit {\n\t\t\treturn amount'),
 ('posted-credit-lost', 'return amount\n\tcase Reserved', 'return 0\n\tcase Reserved'),
 ('reserve-credits', 'case Reserved:\n\t\tif debit {\n\t\t\treturn -amount', 'case Reserved:\n\t\tif debit {\n\t\t\treturn amount'),
 ('capture-wrong-side', 'if !debit {', 'if debit {'),
 ('release-wrong-side', 'case Released:\n\t\tif debit {', 'case Released:\n\t\tif !debit {'),
 ('wrong-internal-target', 'return destination', 'return source'),
 ('wrong-technical-shard', 'source%32 + 1', 'source%32 + 2'),
]
results = []
with tempfile.TemporaryDirectory(prefix='pix-mutations-') as directory:
 target = pathlib.Path(directory)
 shutil.copytree(root/'internal/domain', target/'internal/domain')
 shutil.copy(root/'go.mod', target/'go.mod')
 original = (target/'internal/domain/policy/policy.go').read_text()
 subprocess.run([go,'test','./internal/domain/policy'],cwd=target,check=True,capture_output=True)
 for name,before,after in mutants:
  if before not in original:
   raise RuntimeError(f'Mutation no longer applies: {name}')
  (target/'internal/domain/policy/policy.go').write_text(original.replace(before,after,1))
  compiled=subprocess.run([go,'test','-c','-o',str(target/'mutant.test'),'./internal/domain/policy'],cwd=target,capture_output=True,text=True)
  if compiled.returncode:
   status='invalid'; output=compiled.stderr
  else:
   run=subprocess.run([str(target/'mutant.test')],cwd=target,capture_output=True,text=True,timeout=30)
   status='killed' if run.returncode else 'survived';output=run.stdout
  results.append({'name':name,'status':status,'evidence':output[-1800:]})
report={'scope':'selected financial semantic mutants','total':len(results),'killed':sum(r['status']=='killed' for r in results),'results':results}
(root/'reports').mkdir(exist_ok=True)
(root/'reports/mutations.json').write_text(json.dumps(report,indent=2))
print(json.dumps({k:v for k,v in report.items() if k!='results'}))
raise SystemExit(0 if report['killed']==report['total'] else 1)
