# Test Pack for heredocs in varfiles

Reported Error:

```
nomad-pack render toml_pack -f vars.hcl
! Failed To Process Pack

        Error:   No closing marker was found for the string.
        Type:    *errors.errorString
        Context: 
                 - HCL Range: vars.hcl:5,4-4
                 - Registry Name: dev
                 - Pack Name: toml_pack
                 - Pack Ref: dev
```

Found to be a case where the HEREDOC terminating string was not followed with a
linefeed before the EOF.
