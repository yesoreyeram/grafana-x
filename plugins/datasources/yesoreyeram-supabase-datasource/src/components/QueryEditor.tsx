import React, { ChangeEvent, useCallback, useEffect, useState } from 'react';
import { Button, IconButton, InlineField, InlineFieldRow, InlineSwitch, Input, RadioButtonGroup, Select } from '@grafana/ui';
import { QueryEditorProps, SelectableValue } from '@grafana/data';

import { DataSource } from '../datasource';
import { SupabaseDataSourceOptions, SupabaseQuery, SupabaseQueryType } from '../types';
import { parseSort, serializeSort, SortDirection, SortItem } from '../sort';
import { emptyRootGroup, FilterGroup, parseFilterTree, stringifyFilterTree } from '../filter';
import { FilterEditor } from './FilterEditor';

type Props = QueryEditorProps<DataSource, SupabaseQuery, SupabaseDataSourceOptions>;

const LABEL_WIDTH = 20;

const QUERY_TYPE_OPTIONS: Array<SelectableValue<SupabaseQueryType>> = [
  { label: 'Records', value: 'records', description: 'Return matching records' },
  { label: 'Count', value: 'count', description: 'Return the number of matching records' },
];

const DIRECTION_OPTIONS: Array<SelectableValue<SortDirection>> = [
  { label: 'Asc', value: 'asc' },
  { label: 'Desc', value: 'desc' },
];

export function tableChangePatch(tableId: string, changingTable: boolean): Partial<SupabaseQuery> {
  if (!changingTable) {
    return { tableId };
  }
  return { tableId, select: '', filterTree: '', sort: '' };
}

export function QueryEditor({ query, onChange, onRunQuery, datasource }: Props) {
  const { tableId, sort, select, filterTree, limit, offset } = query;
  const queryType = query.queryType ?? 'records';
  const isCount = queryType === 'count';

  const [tables, setTables] = useState<Array<SelectableValue<string>>>([]);
  const [loadingTables, setLoadingTables] = useState(false);
  const [tablesError, setTablesError] = useState<string | undefined>();

  // Sort is persisted as a JSON array of {field, direction} but edited as rows.
  const [sortItems, setSortItems] = useState<SortItem[]>(() => parseSort(sort));

  useEffect(() => {
    if (serializeSort(sortItems) !== (sort ?? '')) {
      setSortItems(parseSort(sort));
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sort]);

  // Structured filter tree edited via the filter builder. Persisted as JSON.
  const [filterRoot, setFilterRoot] = useState<FilterGroup>(() => parseFilterTree(filterTree));

  useEffect(() => {
    if (stringifyFilterTree(filterRoot) !== (filterTree ?? '')) {
      setFilterRoot(parseFilterTree(filterTree));
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [filterTree]);

  // Load the list of tables from the PostgREST schema.
  useEffect(() => {
    let cancelled = false;
    setLoadingTables(true);
    setTablesError(undefined);
    datasource
      .getTables()
      .then((res) => {
        if (!cancelled) {
          setTables(res.map((t) => ({ label: t.title, value: t.id })));
        }
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
  }, [datasource]);

  const update = useCallback(
    (patch: Partial<SupabaseQuery>) => {
      onChange({ ...query, ...patch });
    },
    [onChange, query]
  );

  const onQueryTypeChange = (value: SupabaseQueryType) => {
    update({ queryType: value });
    onRunQuery();
  };

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

  const onLimitChange = (e: ChangeEvent<HTMLInputElement>) => {
    const n = parseInt(e.target.value, 10);
    update({ limit: isNaN(n) ? 0 : n });
  };

  const onOffsetChange = (e: ChangeEvent<HTMLInputElement>) => {
    const n = parseInt(e.target.value, 10);
    update({ offset: isNaN(n) ? 0 : n });
  };

  const onSelectChange = (e: ChangeEvent<HTMLInputElement>) => {
    update({ select: e.target.value });
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

  const onSortDirectionChange = (index: number, direction: SortDirection) => {
    const next = sortItems.map((item, i) => (i === index ? { ...item, direction } : item));
    applySort(next, true);
  };

  return (
    <div>
      <div className="gf-form">
        <InlineField
          label="Query type"
          labelWidth={LABEL_WIDTH}
          tooltip="Records returns rows; Count returns the number of matching records (respecting filters)."
        >
          <RadioButtonGroup<SupabaseQueryType>
            options={QUERY_TYPE_OPTIONS}
            value={queryType}
            onChange={onQueryTypeChange}
          />
        </InlineField>
      </div>

      <div className="gf-form">
        <InlineField
          label="Table"
          labelWidth={LABEL_WIDTH}
          tooltip="Select a Postgres table or view. The list is fetched from the PostgREST schema."
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
      </div>

      {!isCount && (
        <div className="gf-form">
          <InlineField
            label="Select"
            labelWidth={LABEL_WIDTH}
            tooltip="Comma-separated column names. Leave empty to select all columns."
          >
            <Input
              width={40}
              name="select"
              placeholder="id, name, created_at (leave empty for all)"
              value={select ?? ''}
              onChange={onSelectChange}
              onBlur={onRunQuery}
              disabled={!tableId}
            />
          </InlineField>
        </div>
      )}

      <div className="gf-form" style={{ alignItems: 'flex-start' }}>
        <InlineField
          label="Filters"
          labelWidth={LABEL_WIDTH}
          tooltip="Filter records. Compiled into PostgREST query parameters server-side."
        >
          <FilterEditor group={filterRoot} disabled={!tableId} onChange={onFilterChange} />
        </InlineField>
      </div>

      {!isCount && (
        <>
          <div className="gf-form" style={{ alignItems: 'flex-start' }}>
            <InlineField
              label="Sort"
              labelWidth={LABEL_WIDTH}
              tooltip="Order results by one or more columns. Applied in order."
            >
              <div>
                {sortItems.map((item, index) => (
                  <InlineFieldRow key={index} style={{ marginBottom: 4, alignItems: 'center' }}>
                    <Input
                      width={28}
                      placeholder="Column name"
                      value={item.field}
                      onChange={(e: ChangeEvent<HTMLInputElement>) => {
                        const next = sortItems.map((s, i) =>
                          i === index ? { ...s, field: e.target.value } : s
                        );
                        applySort(next, false);
                      }}
                      onBlur={() => onRunQuery()}
                      disabled={!tableId}
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

          <InlineFieldRow>
            <InlineField
              label="Limit"
              labelWidth={LABEL_WIDTH}
              tooltip="Maximum number of rows. 0 returns all (auto-paginated, 1000 rows/request)."
            >
              <Input width={20} type="number" min={0} value={limit ?? 0} onChange={onLimitChange} onBlur={onRunQuery} />
            </InlineField>

            <InlineField
              label="Offset"
              tooltip="Number of rows to skip."
            >
              <Input width={20} type="number" min={0} value={offset ?? 0} onChange={onOffsetChange} onBlur={onRunQuery} />
            </InlineField>
          </InlineFieldRow>
        </>
      )}
      <InlineFieldRow>
        <InlineField
          label="Hide system fields"
          labelWidth={LABEL_WIDTH}
          tooltip="Hide metadata-style columns (id, created_at/updated_at, underscore-prefixed names, etc.) from the returned frame."
        >
          <InlineSwitch
            value={!!query.hideSystemFields}
            onChange={(e) => {
              update({ hideSystemFields: e.currentTarget.checked });
              onRunQuery();
            }}
          />
        </InlineField>
      </InlineFieldRow>

    </div>
  );
}
