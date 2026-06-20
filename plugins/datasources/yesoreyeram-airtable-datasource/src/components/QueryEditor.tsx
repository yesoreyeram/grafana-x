import React, { ChangeEvent, useCallback, useEffect, useState } from 'react';
import {
  InlineField,
  InlineFieldRow,
  Input,
  Select,
  MultiSelect,
  IconButton,
  RadioButtonGroup,
  Button,
} from '@grafana/ui';
import { QueryEditorProps, SelectableValue } from '@grafana/data';

import { DataSource } from '../datasource';
import { FieldInfo, AirtableDataSourceOptions, AirtableQuery, AirtableQueryType } from '../types';
import { parseSort, serializeSort, SortDirection, SortItem } from '../sort';
import { emptyRootGroup, FilterGroup, parseFilterTree, stringifyFilterTree } from '../filter';
import { FilterEditor } from './FilterEditor';

type Props = QueryEditorProps<DataSource, AirtableQuery, AirtableDataSourceOptions>;

const LABEL_WIDTH = 16;

const QUERY_TYPE_OPTIONS: Array<SelectableValue<AirtableQueryType>> = [
  { label: 'Records', value: 'records', description: 'Return matching records' },
  { label: 'Count', value: 'count', description: 'Return the number of matching records' },
];

const DIRECTION_OPTIONS: Array<SelectableValue<SortDirection>> = [
  { label: 'Asc', value: 'asc' },
  { label: 'Desc', value: 'desc' },
];

/**
 * Build the query patch for a table selection. When the table changes, the
 * table-dependent options (view, filters, sort, fields) are cleared because they
 * reference fields that no longer exist in the new table.
 */
export function tableChangePatch(tableId: string, changingTable: boolean): Partial<AirtableQuery> {
  if (!changingTable) {
    return { tableId };
  }
  return { tableId, viewId: '', filterTree: '', sort: '', fields: '' };
}

export function QueryEditor({ query, onChange, onRunQuery, datasource }: Props) {
  const { tableId, viewId, sort, fields, filterTree, limit } = query;
  const queryType = query.queryType ?? 'records';
  const isCount = queryType === 'count';
  const configuredBaseId = datasource.baseId;
  // When no base id is configured on the datasource, the user picks a base here
  // to scope the table list.
  const showBasePicker = !configuredBaseId;

  // The base to list tables from: the picked one, else the datasource-configured base.
  const selectedBaseId = showBasePicker ? (query.baseId ?? '') : configuredBaseId;
  const baseId = selectedBaseId;

  const [bases, setBases] = useState<Array<SelectableValue<string>>>([]);
  const [loadingBases, setLoadingBases] = useState(false);
  const [basesError, setBasesError] = useState<string | undefined>();

  const [tables, setTables] = useState<Array<SelectableValue<string>>>([]);
  const [loadingTables, setLoadingTables] = useState(false);
  const [tablesError, setTablesError] = useState<string | undefined>();

  const [fieldList, setFieldList] = useState<FieldInfo[]>([]);
  const [fieldOptions, setFieldOptions] = useState<Array<SelectableValue<string>>>([]);
  const [loadingFields, setLoadingFields] = useState(false);
  const [fieldsError, setFieldsError] = useState<string | undefined>();

  const [viewOptions, setViewOptions] = useState<Array<SelectableValue<string>>>([]);
  const [loadingViews, setLoadingViews] = useState(false);
  const [viewsError, setViewsError] = useState<string | undefined>();

  // Sort is persisted as a JSON array of {field, direction} but edited as rows.
  const [sortItems, setSortItems] = useState<SortItem[]>(() => parseSort(sort));

  useEffect(() => {
    if (serializeSort(sortItems) !== (sort ?? '')) {
      setSortItems(parseSort(sort));
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sort]);

  // Structured filter tree edited via the filter builder. Persisted as JSON
  // (filterTree); the formula is built server-side.
  const [filterRoot, setFilterRoot] = useState<FilterGroup>(() => parseFilterTree(filterTree));

  useEffect(() => {
    if (stringifyFilterTree(filterRoot) !== (filterTree ?? '')) {
      setFilterRoot(parseFilterTree(filterTree));
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [filterTree]);

  // Load the list of bases (only when no base id is configured).
  useEffect(() => {
    if (!showBasePicker) {
      return;
    }
    let cancelled = false;
    setLoadingBases(true);
    setBasesError(undefined);
    datasource
      .getBases()
      .then((res) => {
        if (!cancelled) {
          setBases(res.map((b) => ({ label: b.title, value: b.id, description: b.id })));
        }
      })
      .catch((err) => {
        if (!cancelled) {
          setBasesError(err?.data?.error ?? err?.message ?? 'Failed to load bases');
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoadingBases(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [showBasePicker, datasource]);

  // Load the selected base's tables.
  useEffect(() => {
    let cancelled = false;
    if (showBasePicker && !baseId) {
      setTables([]);
      setTablesError(undefined);
      return;
    }
    setLoadingTables(true);
    setTablesError(undefined);
    datasource
      .getTables(baseId || undefined)
      .then((res) => {
        if (cancelled) {
          return;
        }
        setTables(res.map((t) => ({ label: t.title, value: t.id, description: t.id })));
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
  }, [baseId, showBasePicker, datasource]);

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
      .getFields(tableId, baseId || undefined)
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
  }, [tableId, baseId, datasource]);

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
      .getViews(tableId, baseId || undefined)
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
  }, [tableId, baseId, datasource]);

  const update = useCallback(
    (patch: Partial<AirtableQuery>) => {
      onChange({ ...query, ...patch });
    },
    [onChange, query]
  );

  const onQueryTypeChange = (value: AirtableQueryType) => {
    update({ queryType: value });
    onRunQuery();
  };

  const onBaseSelect = (value: SelectableValue<string> | null) => {
    const id = value?.value ?? '';
    if (id === selectedBaseId) {
      return;
    }
    // Changing base invalidates the current table and its dependents.
    setFilterRoot(emptyRootGroup());
    setSortItems([]);
    update({ baseId: id, ...tableChangePatch('', true) });
    onRunQuery();
  };

  const selectedBase: SelectableValue<string> | null = selectedBaseId
    ? (bases.find((b) => b.value === selectedBaseId) ?? { label: selectedBaseId, value: selectedBaseId })
    : null;

  const onTableIdSelect = (value: SelectableValue<string> | null) => {
    const id = value?.value ?? '';
    const changingTable = id !== tableId;
    if (changingTable) {
      setFilterRoot(emptyRootGroup());
      setSortItems([]);
    }
    update(tableChangePatch(id, changingTable));
    onRunQuery();
  };

  const selectedTable: SelectableValue<string> | null = tableId
    ? (tables.find((t) => t.value === tableId) ?? { label: tableId, value: tableId })
    : null;

  const onViewSelect = (value: SelectableValue<string> | null) => {
    update({ viewId: value?.value ?? '' });
    onRunQuery();
  };

  const selectedView: SelectableValue<string> | null = viewId
    ? (viewOptions.find((v) => v.value === viewId) ?? { label: viewId, value: viewId })
    : null;

  const onLimitChange = (e: ChangeEvent<HTMLInputElement>) => {
    const n = parseInt(e.target.value, 10);
    update({ limit: isNaN(n) ? 0 : n });
  };

  const onFilterChange = (root: FilterGroup) => {
    setFilterRoot(root);
    update({ filterTree: stringifyFilterTree(root) });
    onRunQuery();
  };

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

  // `fields` is persisted as a comma-separated string for the Airtable API.
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
          tooltip="Records returns rows; Count returns the number of matching records (respecting filters)."
        >
          <RadioButtonGroup<AirtableQueryType>
            options={QUERY_TYPE_OPTIONS}
            value={queryType}
            onChange={onQueryTypeChange}
          />
        </InlineField>
      </div>

      {showBasePicker && (
        <div className="gf-form">
          <InlineField
            label="Base"
            labelWidth={LABEL_WIDTH}
            tooltip="Select an Airtable base. The list is fetched with the schema.bases:read scope. You can also type a base id (app...) manually."
            error={basesError}
            invalid={!!basesError}
            required
          >
            <Select<string>
              width={40}
              isClearable
              isLoading={loadingBases}
              options={bases}
              value={selectedBase}
              onChange={onBaseSelect}
              allowCustomValue
              placeholder="Select base"
              noOptionsMessage="No bases found"
            />
          </InlineField>
        </div>
      )}

      <InlineFieldRow>
        <InlineField
          label="Table"
          labelWidth={LABEL_WIDTH}
          tooltip="Select an Airtable table. The list is fetched for the selected base. You can also type a table id (tbl...) or name manually."
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
            disabled={showBasePicker && !baseId}
            placeholder={showBasePicker && !baseId ? 'Select a base first' : 'Select table'}
            noOptionsMessage="No tables found"
          />
        </InlineField>

        <InlineField
          label="View"
          tooltip="Optional. Query a specific Airtable view of the table. A view applies its own saved filters, sorts and hidden fields."
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
          tooltip="Filter records. Add individual filters or nested filter groups. Compiled into an Airtable filterByFormula server-side. Operators adapt to each field's type."
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
                      value={
                        item.field
                          ? (fieldOptions.find((o) => o.value === item.field) ?? {
                              label: item.field,
                              value: item.field,
                            })
                          : null
                      }
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
            <InlineField
              label="Limit"
              labelWidth={LABEL_WIDTH}
              tooltip="Maximum number of records. 0 returns all (auto-paginated, 100 records/request)."
            >
              <Input width={20} type="number" min={0} value={limit ?? 0} onChange={onLimitChange} onBlur={onRunQuery} />
            </InlineField>
          </div>
        </>
      )}
    </div>
  );
}
