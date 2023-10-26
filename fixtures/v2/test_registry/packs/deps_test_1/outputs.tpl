{
  "job_name": [[ var "job_name" . | quote ]],
  "child1.job_name": [[ var "job_name" .child1 | quote ]],
  "child1.gc.job_name": [[ var "job_name" .child1.gc | quote ]],
  "child2.job_name": [[ var "job_name" .child2 | quote ]],
  "child2.gc.job_name": [[ var "job_name" .child2.gc | quote ]]
}
