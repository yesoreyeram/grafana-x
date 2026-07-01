import React, { ChangeEvent, useCallback, useEffect, useState } from 'react';
import { Button, IconButton, InlineField, InlineFieldRow, InlineSwitch, Input, MultiSelect, RadioButtonGroup, Select } from '@grafana/ui';
import { QueryEditorProps, SelectableValue } from '@grafana/data';

import { DataSource } from '../datasource';
import { FieldInfo, DirectusDataSourceOptions, DirectusQuery, DirectusQueryType } from '../types';
import { parseSort, serializeSort, SortDirection, SortItem } from '../sort';
import { emptyRootGroup, FilterGroup, parseFilterTree, stringifyFilterTree } from '../filter';
import { FilterEditor } from './FilterEditor';

type Props = QueryEditorProps<DataSource, DirectusQuery, DirectusDataSourceOptions>;

const LABEL_WIDTH = 20;

const QUERY_TYPE_OPTIONS: Array<SelectableValue<DirectusQueryType>> = [
  { label: 'Records', value: 'records', description: 'Return matching records' },
  { label: 'Count', value: 'count', description: 'Return the number of matching records' },
];

const DIRECTION_OPTIONS: Array<SelectableValue<SortDirection>> = [
  { label: 'Asc', value: 'asc' },
  { label: 'Desc', value: 'desc' },
];

function collectionChangePatch(collectionId: string, changingCollection: boolean): Partial<DirectusQuery> {
  if (!changingCollection) {
    return { collectionId };
  }
  return { collectionId, filterTree: '', sort: '', fields: '' };
}

export function QueryEditor({ query, onChange, onRunQuery, datasource }: Props) {
  const { collectionId, sort, fields, filterTree, limit, offset, search } = query;
  const queryType = query.queryType ?? 'records';
  const isCount = queryType === 'count';

  const [collections, setCollections] = useState<Array<SelectableValue<string>>>([]);
  const [loadingCollections, setLoadingCollections] = useState(false);
  const [collectionsError, setCollectionsError] = useState<string | undefined>();

  const [fieldList, setFieldList] = useState<FieldInfo[]>([]);
  const [fieldOptions, setFieldOptions] = useState<Array<SelectableValue<string>>>([]);
  const [loadingFields, setLoadingFields] = useState(false);
  const [fieldsError, setFieldsError] = useState<string | undefined>();

  const [sortItems, setSortItems] = useState<SortItem[]>(() => parseSort(sort));

  useEffect(() => {
    if (serializeSort(sortItems) !== (sort ?? '')) {
      setSortItems(parseSort(sort));
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sort]);

  const [filterRoot, setFilterRoot] = useState<FilterGroup>(() => parseFilterTree(filterTree));

  useEffect(() => {
    if (stringifyFilterTree(filterRoot) !== (filterTree ?? '')) {
      setFilterRoot(parseFilterTree(filterTree));
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [filterTree]);

  // Load collections
  useEffect(() => {
    let cancelled = false;
    setLoadingCollections(true);
    setCollectionsError(undefined);
    datasource
      .getCollections()
      .then((res) => {
        if (!cancelled) {
          setCollections(res.map((c) => ({ label: c.name || c.collection, value: c.collection, description: c.collection })));
        }
      })
      .catch((err) => {
        if (!cancelled) {
          setCollectionsError(err?.data?.error ?? err?.message ?? 'Failed to load collections');
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoadingCollections(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [datasource]);

  // Load fields for the selected collection
  useEffect(() => {
    let cancelled = false;
    if (!collectionId) {
      setFieldList([]);
      setFieldOptions([]);
      setFieldsError(undefined);
      return;
    }
    setLoadingFields(true);
    setFieldsError(undefined);
    datasource
      .getFields(collectionId)
      .then((res) => {
        if (!cancelled) {
          setFieldList(res);
          setFieldOptions(res.map((f) => ({ label: f.field, value: f.field, description: f.type })));
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
  }, [collectionId, datasource]);

  const update = useCallback(
    (patch: Partial<DirectusQuery>) => {
      onChange({ ...query, ...patch });
    },
    [onChange, query]
  );

  const onQueryTypeChange = (value: DirectusQueryType) => {
    update({ queryType: value });
    onRunQuery();
  };

  const onCollectionSelect = (value: SelectableValue<string> | null) => {
    const id = value?.value ?? '';
    const changingCollection = id !== collectionId;
    if (changingCollection) {
      setFilterRoot(emptyRootGroup());
      setSortItems([]);
    }
    update(collectionChangePatch(id, changingCollection));
    onRunQuery();
  };

  const selectedCollection: SelectableValue<string> | null = collectionId
    ? (collections.find((c) => c.value === collectionId) ?? { label: collectionId, value: collectionId })
    : null;

  const onLimitChange = (e: ChangeEvent<HTMLInputElement>) => {
    const n = parseInt(e.target.value, 10);
    update({ limit: isNaN(n) ? 0 : n });
  };

  const onOffsetChange = (e: ChangeEvent<HTMLInputElement>) => {
    const n = parseInt(e.target.value, 10);
    update({ offset: isNaN(n) ? 0 : n });
  };

  const onSearchChange = (e: ChangeEvent<HTMLInputElement>) => {
    update({ search: e.target.value });
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
          <RadioButtonGroup<DirectusQueryType>
            options={QUERY_TYPE_OPTIONS}
            value={queryType}
            onChange={onQueryTypeChange}
          />
        </InlineField>
      </div>

      <div className="gf-form">
        <InlineField
          label="Collection"
          labelWidth={LABEL_WIDTH}
          tooltip="Select a Directus collection. The list is fetched from the Directus API."
          error={collectionsError}
          invalid={!!collectionsError}
          required
        >
          <Select<string>
            width={40}
            isClearable
            isLoading={loadingCollections}
            options={collections}
            value={selectedCollection}
            onChange={onCollectionSelect}
            allowCustomValue
            placeholder="Select collection"
            noOptionsMessage="No collections found"
          />
        </InlineField>
      </div>

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
              placeholder={collectionId ? 'All fields' : 'Select a collection first'}
              disabled={!collectionId}
              noOptionsMessage="No fields found"
            />
          </InlineField>
        </div>
      )}

      {!isCount && (
        <div className="gf-form">
          <InlineField
            label="Search"
            labelWidth={LABEL_WIDTH}
            tooltip="Directus full-text search parameter. Matches against the collection's configured search fields."
          >
            <Input
              width={40}
              value={search ?? ''}
              placeholder="Search text"
              onChange={onSearchChange}
              onBlur={onRunQuery}
              disabled={!collectionId}
            />
          </InlineField>
        </div>
      )}

      <div className="gf-form" style={{ alignItems: 'flex-start' }}>
        <InlineField
          label="Filters"
          labelWidth={LABEL_WIDTH}
          tooltip="Filter records. Add individual filters or nested filter groups. Compiled into a Directus JSON filter object server-side. Operators adapt to each field's type."
        >
          <FilterEditor group={filterRoot} fields={fieldList} disabled={!collectionId} onChange={onFilterChange} />
        </InlineField>
      </div>

      {!isCount && (
        <>
          <div className="gf-form" style={{ alignItems: 'flex-start' }}>
            <InlineField
              label="Sort"
              labelWidth={LABEL_WIDTH}
              tooltip="Order results by one or more fields."
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
                      disabled={!collectionId}
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
                <Button variant="secondary" size="sm" icon="plus" onClick={onAddSort} disabled={!collectionId}>
                  Add sort
                </Button>
              </div>
            </InlineField>
          </div>

          <InlineFieldRow>
            <InlineField
              label="Limit"
              labelWidth={LABEL_WIDTH}
              tooltip="Maximum number of records. 0 returns all (auto-paginated, 100 records/request)."
            >
              <Input width={20} type="number" min={0} value={limit ?? 0} onChange={onLimitChange} onBlur={onRunQuery} />
            </InlineField>

            <InlineField
              label="Offset"
              tooltip="Number of records to skip (offset-based pagination)."
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
