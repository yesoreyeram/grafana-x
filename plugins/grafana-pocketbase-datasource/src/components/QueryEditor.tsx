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
  TextArea,
  InlineSwitch,
} from '@grafana/ui';
import { QueryEditorProps, SelectableValue } from '@grafana/data';

import { DataSource } from '../datasource';
import { FieldInfo, PocketBaseDataSourceOptions, PocketBaseQuery, PocketBaseQueryType } from '../types';
import { parseSort, serializeSort, SortDirection, SortItem } from '../sort';
import { emptyRootGroup, FilterGroup, parseFilterTree, stringifyFilterTree } from '../filter';
import { FilterEditor } from './FilterEditor';

type Props = QueryEditorProps<DataSource, PocketBaseQuery, PocketBaseDataSourceOptions>;

const LABEL_WIDTH = 16;

const QUERY_TYPE_OPTIONS: Array<SelectableValue<PocketBaseQueryType>> = [
  { label: 'Records', value: 'records', description: 'Return matching records' },
  { label: 'Count', value: 'count', description: 'Return the number of matching records' },
];

const DIRECTION_OPTIONS: Array<SelectableValue<SortDirection>> = [
  { label: 'Asc', value: 'asc' },
  { label: 'Desc', value: 'desc' },
];

/**
 * Build the query patch for a collection selection. When the collection changes,
 * the collection-dependent options (filters, sort, fields) are cleared because
 * they reference fields that no longer exist in the new collection.
 */
export function collectionChangePatch(collectionId: string, changingCollection: boolean): Partial<PocketBaseQuery> {
  if (!changingCollection) {
    return { collectionId };
  }
  return { collectionId, filterTree: '', rawFilter: '', sort: '', fields: '' };
}

export function QueryEditor({ query, onChange, onRunQuery, datasource }: Props) {
  const { collectionId, sort, fields, filterTree, rawFilter, limit, hideSystemFields } = query;
  const queryType = query.queryType ?? 'records';
  const isCount = queryType === 'count';

  const [collections, setCollections] = useState<Array<SelectableValue<string>>>([]);
  const [loadingCollections, setLoadingCollections] = useState(false);
  const [collectionsError, setCollectionsError] = useState<string | undefined>();

  const [fieldList, setFieldList] = useState<FieldInfo[]>([]);
  const [fieldOptions, setFieldOptions] = useState<Array<SelectableValue<string>>>([]);
  const [loadingFields, setLoadingFields] = useState(false);
  const [fieldsError, setFieldsError] = useState<string | undefined>();

  // Sort is persisted as a JSON array of {attribute, direction} but edited as rows.
  const [sortItems, setSortItems] = useState<SortItem[]>(() => parseSort(sort));

  useEffect(() => {
    if (serializeSort(sortItems) !== (sort ?? '')) {
      setSortItems(parseSort(sort));
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sort]);

  // Structured filter tree edited via the filter builder. Persisted as JSON
  // (filterTree); the filter expression is built server-side.
  const [filterRoot, setFilterRoot] = useState<FilterGroup>(() => parseFilterTree(filterTree));

  useEffect(() => {
    if (stringifyFilterTree(filterRoot) !== (filterTree ?? '')) {
      setFilterRoot(parseFilterTree(filterTree));
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [filterTree]);

  // Load the list of collections.
  useEffect(() => {
    let cancelled = false;
    setLoadingCollections(true);
    setCollectionsError(undefined);
    datasource
      .getCollections()
      .then((res) => {
        if (!cancelled) {
          setCollections(res.map((c) => ({ label: c.name, value: c.name, description: c.type })));
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

  // Load the selected collection's fields for the multi-select and filters.
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
          setFieldOptions(res.map((f) => ({ label: f.name, value: f.name, description: f.type })));
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
    (patch: Partial<PocketBaseQuery>) => {
      onChange({ ...query, ...patch });
    },
    [onChange, query]
  );

  const onQueryTypeChange = (value: PocketBaseQueryType) => {
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

  const onFilterChange = (root: FilterGroup) => {
    setFilterRoot(root);
    update({ filterTree: stringifyFilterTree(root) });
    onRunQuery();
  };

  const onRawFilterChange = (e: ChangeEvent<HTMLTextAreaElement>) => {
    update({ rawFilter: e.currentTarget.value });
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
    setSortItems([...sortItems, { attribute: '', direction: 'asc' }]);
  };

  const onRemoveSort = (index: number) => {
    applySort(
      sortItems.filter((_, i) => i !== index),
      true
    );
  };

  const onSortAttributeChange = (index: number, value: SelectableValue<string> | null) => {
    const next = sortItems.map((item, i) => (i === index ? { ...item, attribute: value?.value ?? '' } : item));
    applySort(next, true);
  };

  const onSortDirectionChange = (index: number, direction: SortDirection) => {
    const next = sortItems.map((item, i) => (i === index ? { ...item, direction } : item));
    applySort(next, true);
  };

  // `fields` is persisted as a comma-separated string for the `fields` parameter.
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

  const onHideSystemFieldsChange = (e: ChangeEvent<HTMLInputElement>) => {
    update({ hideSystemFields: e.currentTarget.checked });
    onRunQuery();
  };

  const usingRawFilter = !!rawFilter && rawFilter.trim().length > 0;

  return (
    <div>
      <div className="gf-form">
        <InlineField
          label="Query type"
          labelWidth={LABEL_WIDTH}
          tooltip="Records returns rows; Count returns the number of matching records (respecting filters)."
        >
          <RadioButtonGroup<PocketBaseQueryType>
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
          tooltip="Select a PocketBase collection. The list is fetched from the API (requires superuser auth). You can also type a collection id or name manually."
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
            tooltip="Fields (columns) to return, compiled into the PocketBase `fields` parameter. Leave empty to return all fields. The identity fields (id, created, updated) are always included unless 'Hide system fields' is on."
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
          <InlineField
            label="Hide system fields"
            labelWidth={LABEL_WIDTH * 2}
            tooltip="Drop the PocketBase system fields (id, collectionId, collectionName, created, updated) from the result, leaving only your collection's fields."
          >
            <InlineSwitch value={!!hideSystemFields} onChange={onHideSystemFieldsChange} />
          </InlineField>
        </div>
      )}

      <div className="gf-form" style={{ alignItems: 'flex-start' }}>
        <InlineField
          label="Filters"
          labelWidth={LABEL_WIDTH}
          tooltip="Filter records. Add individual filters or nested filter groups. Compiled into a PocketBase filter expression server-side. Operators adapt to each field's type. Ignored when a raw filter is set below."
        >
          <FilterEditor
            group={filterRoot}
            fields={fieldList}
            disabled={!collectionId || usingRawFilter}
            onChange={onFilterChange}
          />
        </InlineField>
      </div>

      <div className="gf-form" style={{ alignItems: 'flex-start' }}>
        <InlineField
          label="Raw filter"
          labelWidth={LABEL_WIDTH}
          tooltip={`Advanced. A raw PocketBase filter expression, for example: status = "active" && total > 10. When set, this takes precedence over the structured filters above.`}
        >
          <TextArea
            width={60}
            rows={2}
            value={rawFilter ?? ''}
            placeholder={`status = "active" && created > "2024-01-01 00:00:00"`}
            onChange={onRawFilterChange}
            onBlur={onRunQuery}
            disabled={!collectionId}
          />
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
                        item.attribute
                          ? (fieldOptions.find((o) => o.value === item.attribute) ?? {
                              label: item.attribute,
                              value: item.attribute,
                            })
                          : null
                      }
                      onChange={(v) => onSortAttributeChange(index, v)}
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

          <div className="gf-form">
            <InlineField
              label="Limit"
              labelWidth={LABEL_WIDTH}
              tooltip="Maximum number of records. 0 returns all (auto-paginated, 200 records/request)."
            >
              <Input width={20} type="number" min={0} value={limit ?? 0} onChange={onLimitChange} onBlur={onRunQuery} />
            </InlineField>
          </div>
        </>
      )}
    </div>
  );
}
