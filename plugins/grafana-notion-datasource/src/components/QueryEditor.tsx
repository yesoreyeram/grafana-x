import React, { ChangeEvent, useCallback, useEffect, useState } from 'react';
import { InlineField, InlineFieldRow, Input, Select, MultiSelect, IconButton, RadioButtonGroup, Button } from '@grafana/ui';
import { QueryEditorProps, SelectableValue } from '@grafana/data';

import { DataSource } from '../datasource';
import { PropertyInfo, NotionDataSourceOptions, NotionQuery, NotionQueryType } from '../types';
import { parseSort, serializeSort, SortDirection, SortItem } from '../sort';
import { emptyRootGroup, FilterGroup, parseFilterTree, stringifyFilterTree } from '../filter';
import { FilterEditor } from './FilterEditor';

type Props = QueryEditorProps<DataSource, NotionQuery, NotionDataSourceOptions>;

const LABEL_WIDTH = 16;

const QUERY_TYPE_OPTIONS: Array<SelectableValue<NotionQueryType>> = [
  { label: 'Records', value: 'records', description: 'Return matching pages' },
  { label: 'Count', value: 'count', description: 'Return the number of matching pages' },
];

const DIRECTION_OPTIONS: Array<SelectableValue<SortDirection>> = [
  { label: 'Asc', value: 'asc' },
  { label: 'Desc', value: 'desc' },
];

/**
 * Build the query patch for a database selection. When the database changes, the
 * database-dependent options (filters, sort, fields) are cleared because they
 * reference properties that no longer exist in the new database.
 */
export function databaseChangePatch(databaseId: string, changingDatabase: boolean): Partial<NotionQuery> {
  if (!changingDatabase) {
    return { databaseId };
  }
  return { databaseId, filterTree: '', sort: '', fields: '' };
}

export function QueryEditor({ query, onChange, onRunQuery, datasource }: Props) {
  const { databaseId, sort, fields, filterTree, limit } = query;
  const queryType = query.queryType ?? 'records';
  const isCount = queryType === 'count';

  const [databases, setDatabases] = useState<Array<SelectableValue<string>>>([]);
  const [loadingDatabases, setLoadingDatabases] = useState(false);
  const [databasesError, setDatabasesError] = useState<string | undefined>();

  const [fieldList, setFieldList] = useState<PropertyInfo[]>([]);
  const [fieldOptions, setFieldOptions] = useState<Array<SelectableValue<string>>>([]);
  const [loadingFields, setLoadingFields] = useState(false);
  const [fieldsError, setFieldsError] = useState<string | undefined>();

  // Sort is persisted on the query as a sort string (e.g. `-Created,Name`) but
  // edited as structured rows. We keep the rows in local state so that
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
  // as JSON (filterTree); the Notion filter object is built server-side from it.
  const [filterRoot, setFilterRoot] = useState<FilterGroup>(() => parseFilterTree(filterTree));

  useEffect(() => {
    if (stringifyFilterTree(filterRoot) !== (filterTree ?? '')) {
      setFilterRoot(parseFilterTree(filterTree));
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [filterTree]);

  useEffect(() => {
    let cancelled = false;
    setLoadingDatabases(true);
    setDatabasesError(undefined);
    datasource
      .getDatabases()
      .then((res) => {
        if (cancelled) {
          return;
        }
        setDatabases(res.map((d) => ({ label: d.title, value: d.id, description: d.id })));
      })
      .catch((err) => {
        if (!cancelled) {
          setDatabasesError(err?.data?.error ?? err?.message ?? 'Failed to load databases');
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoadingDatabases(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [datasource]);

  // Load the selected database's properties for the multi-select.
  useEffect(() => {
    let cancelled = false;
    if (!databaseId) {
      setFieldList([]);
      setFieldOptions([]);
      setFieldsError(undefined);
      return;
    }
    setLoadingFields(true);
    setFieldsError(undefined);
    datasource
      .getProperties(databaseId)
      .then((res) => {
        if (!cancelled) {
          setFieldList(res);
          setFieldOptions(res.map((f) => ({ label: f.title, value: f.title, description: f.type })));
        }
      })
      .catch((err) => {
        if (!cancelled) {
          setFieldsError(err?.data?.error ?? err?.message ?? 'Failed to load properties');
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
  }, [databaseId, datasource]);

  const update = useCallback(
    (patch: Partial<NotionQuery>) => {
      onChange({ ...query, ...patch });
    },
    [onChange, query]
  );

  const onQueryTypeChange = (value: NotionQueryType) => {
    update({ queryType: value });
    onRunQuery();
  };

  const onDatabaseSelect = (value: SelectableValue<string> | null) => {
    const id = value?.value ?? '';
    const changingDatabase = id !== databaseId;
    if (changingDatabase) {
      // Switching databases: the old filters/sort/fields reference properties
      // that no longer exist, so clear the local UI state too.
      setFilterRoot(emptyRootGroup());
      setSortItems([]);
    }
    update(databaseChangePatch(id, changingDatabase));
    onRunQuery();
  };

  // Ensure a manually-typed / restored database id is always selectable even if
  // it isn't part of the fetched options (e.g. permissions).
  const selectedDatabase: SelectableValue<string> | null = databaseId
    ? databases.find((d) => d.value === databaseId) ?? { label: databaseId, value: databaseId }
    : null;

  const onLimitChange = (e: ChangeEvent<HTMLInputElement>) => {
    const n = parseInt(e.target.value, 10);
    update({ limit: isNaN(n) ? 0 : n });
  };

  // Apply filter builder changes: keep the structured tree in local state and
  // persist it as JSON. The Notion filter object is built server-side from it.
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

  // `fields` is persisted as a comma-separated string of property names.
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
          tooltip="Records returns pages; Count returns the number of matching pages (respecting filters)."
        >
          <RadioButtonGroup<NotionQueryType>
            options={QUERY_TYPE_OPTIONS}
            value={queryType}
            onChange={onQueryTypeChange}
          />
        </InlineField>
      </div>

      <InlineFieldRow>
        <InlineField
          label="Database"
          labelWidth={LABEL_WIDTH}
          tooltip="Select a Notion database. The list is fetched from databases shared with your integration. You can also type a database id manually."
          error={databasesError}
          invalid={!!databasesError}
          required
        >
          <Select<string>
            width={40}
            isClearable
            isLoading={loadingDatabases}
            options={databases}
            value={selectedDatabase}
            onChange={onDatabaseSelect}
            allowCustomValue
            placeholder="Select database"
            noOptionsMessage="No databases found"
          />
        </InlineField>
      </InlineFieldRow>

      {!isCount && (
        <div className="gf-form">
          <InlineField
            label="Properties"
            labelWidth={LABEL_WIDTH}
            tooltip="Properties to return. Leave empty to return all properties."
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
              placeholder={databaseId ? 'All properties' : 'Select a database first'}
              disabled={!databaseId}
              noOptionsMessage="No properties found"
            />
          </InlineField>
        </div>
      )}

      <div className="gf-form" style={{ alignItems: 'flex-start' }}>
        <InlineField
          label="Filters"
          labelWidth={LABEL_WIDTH}
          tooltip="Filter pages. Add individual filters or nested filter groups. Operators adapt to each property's type."
        >
          <FilterEditor group={filterRoot} fields={fieldList} disabled={!databaseId} onChange={onFilterChange} />
        </InlineField>
      </div>

      {!isCount && (
        <>
          <div className="gf-form" style={{ alignItems: 'flex-start' }}>
            <InlineField
              label="Sort"
              labelWidth={LABEL_WIDTH}
              tooltip="Order results by one or more properties. Rows are applied in order."
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
                      placeholder="Select property"
                      disabled={!databaseId}
                      noOptionsMessage="No properties found"
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
                <Button variant="secondary" size="sm" icon="plus" onClick={onAddSort} disabled={!databaseId}>
                  Add sort
                </Button>
              </div>
            </InlineField>
          </div>

          <div className="gf-form">
            <InlineField label="Limit" labelWidth={LABEL_WIDTH} tooltip="Maximum number of pages. 0 returns all (auto-paginated).">
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
