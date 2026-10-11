# Routing form and JSON synchronization

## Keep these mappings

| UI action | JSON result |
| --- | --- |
| Inherit weights | Omit allocations |
| Custom weights | deployment_id / weight array |
| Switch to nonweighted policy | Remove allocations |
| Clear fallback for a source | Keep source mapped to [] |
| Delete mapping | Restore inheritance |
| Save group dialog | Update draft only; template submission applies it |

Invalid JSON prevents save and switching to the form. Do not turn zero into one or empty input into zero. Async loads must not overwrite user drafts.

~~~bash
cd frontend
npm run test:unit -- src/components/route_templates/templateForm.test.ts
~~~

[Complete JSON](routing.md) · [Form source](../../../frontend/src/components/route_templates)
