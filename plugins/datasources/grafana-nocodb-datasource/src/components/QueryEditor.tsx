import React, { ChangeEvent, useCallback, useEffect, useState } from 'react';
import { InlineField, InlineFieldRow, Input, Select, MultiSelect, IconButton, RadioButtonGroup, Button } from '@grafana/ui';
import { QueryEditorProps, SelectableValue } from '@grafana/data';

import { DataSource } from '../datasource';
import { FieldInfo, NocoDBDataSourceOptions, NocoDBQuery, NocoDBQueryType, TableInfo } from '../types';
import { parseSort, serializeSort, SortDirection, SortItem } from '../sort';
import { emptyRootGroup, FilterGroup, parseFilterTree, stringifyFilterTree } from '../filter';
import { FilterEditor } from './FilterEditor';

type Props = QueryEditorProps<DataSource, NocoDBQuery, NocoDBDataSourceOptions>;

const LABEL_WIDTH = 16;

const QUERY_TYPE_OPTIONS: Array<SelectableValue<NocoDBQueryType>> = [
  { label: 'Records', value: 'records', description: 'Return matching rows' },
  { label: 'Count', value: 'count', description: 'Return the number of matching rows' },
];

const DIRECTION_OPTIONS: Array<SelectableValue<SortDirection>> = [
  { label: 'Asc', value: 'asc' },
  { label: 'Desc', value: 'desc' },
];

/**
 * Build the query patch for a table selection. When the table changes, the
 * table-dependent options (view, filters, sort, fields) are cleared because they
 * reference columns that no longer exist in the new table.
 */
export function tableChangePatch(tableId: string, baseId: string, changingTable: boolean): Partial<NocoDBQuery> {
  if (!changingTable) {
    return { tableId, baseId };
  }
  return { tableId, baseId, viewId: '', filterTree: '', sort: '', fields: '' };
}

export function QueryEditor({ query, onChange, onRunQuery, datasource }: Props) {
  const { tableId, viewId, sort, fields, filterTree, limit } = query;
  const queryType = query.queryType ?? 'records';
  const isCount = queryType === 'count';
  const baseId = datasource.baseId;

  const [tables, setTables] = useState<Array<SelectableValue<string>>>([]);
  const [tableList, setTableList] = useState<TableInfo[]>([]);
  const [loadingTables, setLoadingTables] = useState(false);
  const [tablesError, setTablesError] = useState<string | undefined>();

  const [fieldList, setFieldList] = useState<FieldInfo[]>([]);
  const [fieldOptions, setFieldOptions] = useState<Array<SelectableValue<string>>>([]);
  const [loadingFields, setLoadingFields] = useState(false);
  const [fieldsError, setFieldsError] = useState<string | undefined>();

  const [viewOptions, setViewOptions] = useState<Array<SelectableValue<string>>>([]);
  const [loadingViews, setLoadingViews] = useState(false);
  const [viewsError, setViewsError] = useState<string | undefined>();

  // Sort is persisted on the query as a NocoDB sort string (e.g. `-CreatedAt,Title`)
  // but edited as structured rows. We keep the rows in local state so that
  // in-progress rows (e.g. a freshly added row with no field yet) can exist in
  // the UI even though they aren't representable in the serialized string.
  const [sortItems, setSortItems] = useState<SortItem[]>(() => parseSort(sort));

  // Re-sync local rows when the persisted sort changes from outside this editor
  // (variable substitution, query history, duplicating a panel, etc.), but
  // avoid clobbering in-progress edits when the change originated here.
  useEffect(() => {
    if (serializeSort(sortItems) !== (sort ?? '')) {
      setSortItems(parseSort(sort));
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sort]);

  // Structured filter tree edited via the filter builder. Persisted on the query
  // as JSON (filterTree); the NocoDB where clause is built server-side from it.
  const [filterRoot, setFilterRoot] = useState<FilterGroup>(() => parseFilterTree(filterTree));

  useEffect(() => {
    if (stringifyFilterTree(filterRoot) !== (filterTree ?? '')) {
      setFilterRoot(parseFilterTree(filterTree));
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [filterTree]);

  useEffect(() => {
    let cancelled = false;
    setLoadingTables(true);
    setTablesError(undefined);
    datasource
      .getTables(baseId || undefined)
      .then((res) => {
        if (cancelled) {
          return;
        }
        setTableList(res);
        setTables(
          res.map((t) => ({
            label: t.baseTitle ? `${t.baseTitle} / ${t.title}` : t.title,
            value: t.id,
            description: t.id,
          }))
        );
      })
      .catch((err) => {
        if (!cancelled) {
          setTablesError(err?.data?.error ?? err?.message ?? 'Failed to load tables');
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoadingTables(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [baseId, datasource]);

  // Load the selected table's fields for the multi-select.
  useEffect(() => {
    let cancelled = false;
    if (!tableId) {
      setFieldList([]);
      setFieldOptions([]);
      setFieldsError(undefined);
      return;
    }
    setLoadingFields(true);
    setFieldsError(undefined);
    datasource
      .getFields(tableId)
      .then((res) => {
        if (!cancelled) {
          setFieldList(res);
          setFieldOptions(res.map((f) => ({ label: f.title, value: f.title, description: f.type })));
        }
      })
      .catch((err) => {
        if (!cancelled) {
          setFieldsError(err?.data?.error ?? err?.message ?? 'Failed to load fields');
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoadingFields(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [tableId, datasource]);

  // Load the selected table's views for the view dropdown.
  useEffect(() => {
    let cancelled = false;
    if (!tableId) {
      setViewOptions([]);
      setViewsError(undefined);
      return;
    }
    setLoadingViews(true);
    setViewsError(undefined);
    datasource
      .getViews(tableId)
      .then((res) => {
        if (!cancelled) {
          setViewOptions(res.map((v) => ({ label: v.title, value: v.id, description: v.id })));
        }
      })
      .catch((err) => {
        if (!cancelled) {
          setViewsError(err?.data?.error ?? err?.message ?? 'Failed to load views');
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoadingViews(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [tableId, datasource]);

  const update = useCallback(
    (patch: Partial<NocoDBQuery>) => {
      onChange({ ...query, ...patch });
    },
    [onChange, query]
  );

  const onQueryTypeChange = (value: NocoDBQueryType) => {
    update({ queryType: value });
    onRunQuery();
  };

  const onTableIdSelect = (value: SelectableValue<string> | null) => {
    const id = value?.value ?? '';
    // Capture the base id of the picked table (required by the v3 data API), with
    // the configured default base as a fallback.
    const resolvedBaseId = tableList.find((t) => t.id === id)?.baseId ?? baseId ?? '';

    const changingTable = id !== tableId;
    if (changingTable) {
      // Switching tables: the old view/filters/sort/fields reference columns that
      // no longer exist, so clear the local UI state too.
      setFilterRoot(emptyRootGroup());
      setSortItems([]);
    }
    update(tableChangePatch(id, resolvedBaseId, changingTable));
    onRunQuery();
  };

  // Ensure a manually-typed / restored table id is always selectable even if it
  // isn't part of the fetched options (e.g. permissions, or a different base).
  const selectedTable: SelectableValue<string> | null = tableId
    ? tables.find((t) => t.value === tableId) ?? { label: tableId, value: tableId }
    : null;

  const onViewSelect = (value: SelectableValue<string> | null) => {
    update({ viewId: value?.value ?? '' });
    onRunQuery();
  };

  const selectedView: SelectableValue<string> | null = viewId
    ? viewOptions.find((v) => v.value === viewId) ?? { label: viewId, value: viewId }
    : null;

  const onLimitChange = (e: ChangeEvent<HTMLInputElement>) => {
    const n = parseInt(e.target.value, 10);
    update({ limit: isNaN(n) ? 0 : n });
  };

  // Apply filter builder changes: keep the structured tree in local state and
  // persist it as JSON. The NocoDB where clause is built server-side from it.
  const onFilterChange = (root: FilterGroup) => {
    setFilterRoot(root);
    update({ filterTree: stringifyFilterTree(root) });
    onRunQuery();
  };

  // Apply local sort rows: update local UI state, and persist the serialized
  // (valid-rows-only) string to the query. Optionally re-run the query.
  const applySort = (items: SortItem[], run: boolean) => {
    setSortItems(items);
    const serialized = serializeSort(items);
    if (serialized !== (sort ?? '')) {
      update({ sort: serialized });
      if (run) {
        onRunQuery();
      }
    }
  };

  const onAddSort = () => {
    // Add an empty row; it lives in local state until a field is chosen.
    setSortItems([...sortItems, { field: '', direction: 'asc' }]);
  };

  const onRemoveSort = (index: number) => {
    applySort(
      sortItems.filter((_, i) => i !== index),
      true
    );
  };

  const onSortFieldChange = (index: number, value: SelectableValue<string> | null) => {
    const next = sortItems.map((item, i) => (i === index ? { ...item, field: value?.value ?? '' } : item));
    applySort(next, true);
  };

  const onSortDirectionChange = (index: number, direction: SortDirection) => {
    const next = sortItems.map((item, i) => (i === index ? { ...item, direction } : item));
    applySort(next, true);
  };

  // `fields` is persisted as a comma-separated string for the NocoDB API.
  const selectedFields: Array<SelectableValue<string>> = (fields ?? '')
    .split(',')
    .map((f) => f.trim())
    .filter((f) => f.length > 0)
    .map((f) => fieldOptions.find((o) => o.value === f) ?? { label: f, value: f });

  const onFieldsSelect = (values: Array<SelectableValue<string>>) => {
    const list = values.map((v) => v.value).filter((v): v is string => !!v);
    update({ fields: list.join(',') });
    onRunQuery();
  };

  return (
    <div>
      <div className="gf-form">
        <InlineField
          label="Query type"
          labelWidth={LABEL_WIDTH}
          tooltip="Records returns rows; Count returns the number of matching rows (respecting filters)."
        >
          <RadioButtonGroup<NocoDBQueryType>
            options={QUERY_TYPE_OPTIONS}
            value={queryType}
            onChange={onQueryTypeChange}
          />
        </InlineField>
      </div>

      <InlineFieldRow>
        <InlineField
          label="Table"
          labelWidth={LABEL_WIDTH}
          tooltip="Select a NocoDB table. The list is fetched from the API. You can also type a table id (prefixed with m) manually."
          error={tablesError}
          invalid={!!tablesError}
          required
        >
          <Select<string>
            width={40}
            isClearable
            isLoading={loadingTables}
            options={tables}
            value={selectedTable}
            onChange={onTableIdSelect}
            allowCustomValue
            placeholder="Select table"
            noOptionsMessage="No tables found"
          />
        </InlineField>

        <InlineField
          label="View"
          tooltip="Optional. Query a specific NocoDB view of the table. A view applies its own saved filters, sorts and hidden fields."
          error={viewsError}
          invalid={!!viewsError}
        >
          <Select<string>
            width={40}
            isClearable
            isLoading={loadingViews}
            options={viewOptions}
            value={selectedView}
            onChange={onViewSelect}
            allowCustomValue
            placeholder={tableId ? 'Default view' : 'Select a table first'}
            disabled={!tableId}
            noOptionsMessage="No views found"
          />
        </InlineField>
      </InlineFieldRow>

      {!isCount && (
        <div className="gf-form">
          <InlineField
            label="Fields"
            labelWidth={LABEL_WIDTH}
            tooltip="Fields to return. Leave empty to return all fields."
            error={fieldsError}
            invalid={!!fieldsError}
          >
            <MultiSelect<string>
              width={40}
              isLoading={loadingFields}
              options={fieldOptions}
              value={selectedFields}
              onChange={onFieldsSelect}
              allowCustomValue
              placeholder={tableId ? 'All fields' : 'Select a table first'}
              disabled={!tableId}
              noOptionsMessage="No fields found"
            />
          </InlineField>
        </div>
      )}

      <div className="gf-form" style={{ alignItems: 'flex-start' }}>
        <InlineField
          label="Filters"
          labelWidth={LABEL_WIDTH}
          tooltip="Filter records. Add individual filters or nested filter groups. Operators adapt to each field's type."
        >
          <FilterEditor group={filterRoot} fields={fieldList} disabled={!tableId} onChange={onFilterChange} />
        </InlineField>
      </div>

      {!isCount && (
        <>
          <div className="gf-form" style={{ alignItems: 'flex-start' }}>
            <InlineField
              label="Sort"
              labelWidth={LABEL_WIDTH}
              tooltip="Order results by one or more fields. Rows are applied in order."
            >
              <div>
                {sortItems.map((item, index) => (
                  <InlineFieldRow key={index} style={{ marginBottom: 4, alignItems: 'center' }}>
                    <Select<string>
                      width={28}
                      options={fieldOptions}
                      value={item.field ? fieldOptions.find((o) => o.value === item.field) ?? { label: item.field, value: item.field } : null}
                      onChange={(v) => onSortFieldChange(index, v)}
                      allowCustomValue
                      placeholder="Select field"
                      disabled={!tableId}
                      noOptionsMessage="No fields found"
                    />
                    <RadioButtonGroup<SortDirection>
                      options={DIRECTION_OPTIONS}
                      value={item.direction}
                      onChange={(v) => onSortDirectionChange(index, v)}
                    />
                    <IconButton
                      name="trash-alt"
                      tooltip="Remove sort"
                      aria-label="Remove sort"
                      onClick={() => onRemoveSort(index)}
                    />
                  </InlineFieldRow>
                ))}
                <Button variant="secondary" size="sm" icon="plus" onClick={onAddSort} disabled={!tableId}>
                  Add sort
                </Button>
              </div>
            </InlineField>
          </div>

          <div className="gf-form">
            <InlineField label="Limit" labelWidth={LABEL_WIDTH} tooltip="Maximum number of records. 0 returns all (auto-paginated).">
              <Input
                width={20}
                type="number"
                min={0}
                value={limit ?? 0}
                onChange={onLimitChange}
                onBlur={onRunQuery}
              />
            </InlineField>
          </div>
        </>
      )}
    </div>
  );
}
